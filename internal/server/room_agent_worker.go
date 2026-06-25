package server

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/simonski/ticket/internal/store"
)

const (
	// roomAgentTaskTTLSeconds is how long a finished ephemeral task is retained
	// before the worker purges it, so the live panel can show recent results.
	roomAgentTaskTTLSeconds = 3600
	// roomAgentTaskPoll is the idle poll cadence; enqueue also wakes the worker.
	roomAgentTaskPoll = 3 * time.Second
)

// roomTaskSignal wakes the dispatcher when a task is enqueued or finishes, so the
// queue drains promptly instead of waiting for the poll tick.
var roomTaskSignal = make(chan struct{}, 1)

func signalRoomTaskWorker() {
	select {
	case roomTaskSignal <- struct{}{}:
	default:
	}
}

// runRoomAgentTaskWorker is the dispatcher for the ephemeral agent backlog
// (TK-168). It repeatedly claims eligible queued tasks — at most one running per
// (room, agent) — runs each concurrently, and periodically purges finished items.
func (s *Server) runRoomAgentTaskWorker(db *sql.DB, hub *liveHub) {
	poll := time.NewTicker(roomAgentTaskPoll)
	defer poll.Stop()
	purge := time.NewTicker(2 * time.Minute)
	defer purge.Stop()
	for {
		// Drain everything currently eligible; each claim marks a task running so
		// the same (room, agent) is not picked again until it finishes.
		for {
			select {
			case <-s.stopRoomWorker:
				return
			default:
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			task, ok, err := store.ClaimNextRoomAgentTask(ctx, db)
			cancel()
			if err != nil {
				log.Printf("server: claim room agent task: %v", err)
				break
			}
			if !ok {
				break
			}
			broadcastRoomQueue(context.Background(), db, hub, task.RoomID)
			go s.executeRoomAgentTask(db, hub, task)
		}
		select {
		case <-s.stopRoomWorker:
			return
		case <-roomTaskSignal:
		case <-poll.C:
		case <-purge.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if _, err := store.PurgeFinishedRoomAgentTasks(ctx, db, roomAgentTaskTTLSeconds); err != nil {
				log.Printf("server: purge room agent tasks: %v", err)
			}
			cancel()
		}
	}
}

// executeRoomAgentTask runs a single claimed task: it generates the agent reply,
// posts the result into the room, marks the task done/failed, and refreshes the
// live queue. It always signals the dispatcher so the next item for the same
// (room, agent) can be picked up.
func (s *Server) executeRoomAgentTask(db *sql.DB, hub *liveHub, task store.RoomAgentTask) {
	defer signalRoomTaskWorker()
	ctx, cancel := context.WithTimeout(context.Background(), 130*time.Second)
	defer cancel()

	room, err := store.GetRoom(ctx, db, task.RoomID)
	if err != nil {
		if _, ferr := store.FinishRoomAgentTask(ctx, db, task.ID, store.RoomAgentTaskFailed, "room not found"); ferr != nil {
			log.Printf("server: finish room agent task: %v", ferr)
		}
		broadcastRoomQueue(ctx, db, hub, task.RoomID)
		return
	}
	agent, aerr := store.GetUserByID(ctx, db, task.AgentID)
	if aerr != nil {
		if _, ferr := store.FinishRoomAgentTask(ctx, db, task.ID, store.RoomAgentTaskFailed, "agent not found"); ferr != nil {
			log.Printf("server: finish room agent task: %v", ferr)
		}
		broadcastRoomQueue(ctx, db, hub, task.RoomID)
		return
	}

	cfg, _ := store.ResolveAgentModelConfig(ctx, db, task.RequesterID, room.ProjectID)
	cmdArgs := resolveAgentChatCommand(ctx, db)
	history, _ := store.ListRoomMessages(ctx, db, room.ID, 20, 0)
	reply, rerr := roomAgentReply(ctx, cfg, cmdArgs, agent.Username, task.Instruction, history)
	if rerr != nil {
		log.Printf("server: room agent task %d failed: %v", task.ID, rerr)
		postAgentNotice(ctx, db, room, agent, "task failed — "+task.Instruction+" (see server logs).", hub)
		if _, ferr := store.FinishRoomAgentTask(ctx, db, task.ID, store.RoomAgentTaskFailed, rerr.Error()); ferr != nil {
			log.Printf("server: finish room agent task: %v", ferr)
		}
		broadcastRoomQueue(ctx, db, hub, task.RoomID)
		return
	}

	reply = strings.TrimSpace(reply)
	// Strip a malformed render block before persistence (TK-177); a valid one is kept
	// for the front-end renderer.
	reply = strings.TrimSpace(sanitizeRenderBlock(reply))
	if reply != "" {
		postAgentText(ctx, db, room, agent, reply, cfg.Model, hub)
	}
	if _, ferr := store.FinishRoomAgentTask(ctx, db, task.ID, store.RoomAgentTaskDone, reply); ferr != nil {
		log.Printf("server: finish room agent task: %v", ferr)
	}
	broadcastRoomQueue(ctx, db, hub, task.RoomID)
}

// postAgentText posts a normal agent chat message (with the model annotation) and
// broadcasts it. Shared by one-shot replies and queue task results.
func postAgentText(ctx context.Context, db *sql.DB, room store.Room, agent store.User, text, model string, hub *liveHub) {
	attrs := store.Attrs{"agent": true}
	if strings.TrimSpace(model) != "" {
		attrs["model"] = model
	}
	out, err := store.PostRoomMessage(ctx, db, store.RoomMessage{
		RoomID:   room.ID,
		SenderID: agent.ID,
		Kind:     "text",
		Body:     text,
		Attrs:    attrs,
	})
	if err != nil {
		log.Printf("server: post agent task result: %v", err)
		return
	}
	if hub != nil {
		hub.broadcastRoomMessage(room.ID, out)
	}
}

// broadcastRoomQueue pushes the room's current agent task list to subscribers.
func broadcastRoomQueue(ctx context.Context, db *sql.DB, hub *liveHub, roomID int64) {
	if hub == nil || roomID == 0 {
		return
	}
	tasks, err := store.ListRoomAgentTasks(ctx, db, roomID)
	if err != nil {
		return
	}
	hub.broadcast(liveEvent{Type: "room_queue", RoomID: roomID, Payload: tasks})
}
