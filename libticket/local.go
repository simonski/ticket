package libticket

import (
	"database/sql"
	"errors"
	"os"
	"strings"

	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
)

func resolveRequestLifecycle(status, stage, state string) (string, string, error) {
	if strings.TrimSpace(stage) != "" || strings.TrimSpace(state) != "" {
		return stage, state, nil
	}
	if strings.TrimSpace(status) == "" {
		return stage, state, nil
	}
	return store.ParseLifecycleStatus(status)
}

type LocalService struct {
	cfg config.Config
}

func NewLocal(cfg config.Config) *LocalService {
	return &LocalService{cfg: cfg}
}

func (s *LocalService) Status() (StatusResponse, error) {
	path, err := config.ResolveDatabasePath()
	if err != nil {
		return StatusResponse{}, err
	}
	if _, err := os.Stat(path); err != nil {
		return StatusResponse{}, err
	}
	db, err := store.Open(path)
	if err != nil {
		return StatusResponse{}, err
	}
	defer db.Close()
	user, err := store.GetUserByUsername(db, LocalUsername())
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return StatusResponse{Status: "ok", Authenticated: false}, nil
	case err != nil:
		return StatusResponse{}, err
	case !user.Enabled:
		return StatusResponse{Status: "ok", Authenticated: false}, nil
	}
	return StatusResponse{
		Status:        "ok",
		Authenticated: true,
		User:          &user,
	}, nil
}

func (s *LocalService) Register(username, password string) (store.User, error) {
	return store.User{}, errors.New("ticket register requires TICKET_MODE=remote")
}

func (s *LocalService) Login(username, password string) (store.User, string, error) {
	return store.User{}, "", errors.New("ticket login requires TICKET_MODE=remote")
}

func (s *LocalService) Logout() error {
	return errors.New("ticket logout requires TICKET_MODE=remote")
}

func (s *LocalService) Count(projectID *int64) (CountSummary, error) {
	db, err := s.openDB()
	if err != nil {
		return CountSummary{}, err
	}
	defer db.Close()
	return store.CountEverything(db, projectID)
}

func (s *LocalService) CreateUser(username, password string) (store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return store.User{}, err
	}
	defer db.Close()
	return store.CreateUser(db, username, password, "user")
}

func (s *LocalService) SetUserEnabled(username string, enabled bool) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.SetUserEnabled(db, username, enabled)
}

func (s *LocalService) ListUsers() ([]store.User, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListUsers(db)
}

func (s *LocalService) DeleteUser(username string) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteUser(db, username)
}

func (s *LocalService) CreateProject(request ProjectCreateRequest) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Project{}, err
	}
	return store.CreateProjectWithParams(db, store.ProjectCreateParams{
		Prefix:             request.Prefix,
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		Notes:              request.Notes,
		CreatedBy:          user.ID,
	})
}

func (s *LocalService) ListProjects() ([]store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListProjects(db)
}

func (s *LocalService) GetProject(id string) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.GetProject(db, id)
}

func (s *LocalService) UpdateProject(id int64, request ProjectUpdateRequest) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.UpdateProjectWithParams(db, id, store.ProjectUpdateParams{
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		Notes:              request.Notes,
	})
}

func (s *LocalService) SetProjectEnabled(id int64, enabled bool) (store.Project, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Project{}, err
	}
	defer db.Close()
	return store.SetProjectStatus(db, id, enabled)
}

func (s *LocalService) CreateTicket(request TicketCreateRequest) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	stage, state, err := resolveRequestLifecycle(request.Status, request.Stage, request.State)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.CreateTicket(db, store.TicketCreateParams{
		ProjectID:          request.ProjectID,
		ParentID:           request.ParentID,
		CloneOf:            request.CloneOf,
		Type:               request.Type,
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		Priority:           request.Priority,
		EstimateEffort:     request.EstimateEffort,
		EstimateComplete:   request.EstimateComplete,
		Assignee:           request.Assignee,
		Stage:              stage,
		State:              state,
		CreatedBy:          user.ID,
	})
}

func (s *LocalService) ListTickets(projectID int64) ([]store.Ticket, error) {
	return s.ListTicketsFiltered(projectID, "", "", "", "", "", "", 0)
}

func (s *LocalService) ListTicketsFiltered(projectID int64, taskType, stage, state, status, search, assignee string, limit int) ([]store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListTickets(db, store.TicketListParams{
		ProjectID: projectID,
		Type:      taskType,
		Stage:     stage,
		State:     state,
		Status:    status,
		Search:    search,
		Assignee:  assignee,
		Limit:     limit,
	})
}

func (s *LocalService) UpdateTicket(id int64, request TicketUpdateRequest) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	stage, state, err := resolveRequestLifecycle(request.Status, request.Stage, request.State)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.UpdateTicket(db, id, store.TicketUpdateParams{
		Title:              request.Title,
		Description:        request.Description,
		AcceptanceCriteria: request.AcceptanceCriteria,
		ParentID:           request.ParentID,
		Assignee:           request.Assignee,
		Stage:              stage,
		State:              state,
		Priority:           request.Priority,
		Order:              request.Order,
		EstimateEffort:     request.EstimateEffort,
		EstimateComplete:   request.EstimateComplete,
		UpdatedBy:          user.ID,
		ActorUsername:      user.Username,
		// Local mode bypasses server-side ownership restrictions.
		ActorRole: "admin",
	})
}

func (s *LocalService) DeleteTicket(id int64) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteTicket(db, id)
}

func (s *LocalService) SetTicketParent(id, parentID int64) (store.Ticket, error) {
	current, err := s.GetTicketByID(id)
	if err != nil {
		return store.Ticket{}, err
	}
	return s.UpdateTicket(id, TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           &parentID,
		Assignee:           current.Assignee,
		Stage:              current.Stage,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	})
}

func (s *LocalService) SetTicketHealth(id int64, score int) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.SetTicketHealth(db, id, score)
}

func (s *LocalService) UnsetTicketParent(id int64) (store.Ticket, error) {
	current, err := s.GetTicketByID(id)
	if err != nil {
		return store.Ticket{}, err
	}
	return s.UpdateTicket(id, TicketUpdateRequest{
		Title:              current.Title,
		Description:        current.Description,
		AcceptanceCriteria: current.AcceptanceCriteria,
		ParentID:           nil,
		Assignee:           current.Assignee,
		Stage:              current.Stage,
		State:              current.State,
		Priority:           current.Priority,
		Order:              current.Order,
		EstimateEffort:     current.EstimateEffort,
		EstimateComplete:   current.EstimateComplete,
	})
}

func (s *LocalService) GetTicketByID(id int64) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.GetTicket(db, id)
}

func (s *LocalService) GetTicket(ref string) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	return store.GetTicketByRef(db, ref)
}

func (s *LocalService) CloneTicket(id int64) (store.Ticket, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Ticket{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Ticket{}, err
	}
	return store.CloneTicket(db, id, user.ID)
}

func (s *LocalService) ListHistory(id int64) ([]store.HistoryEvent, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListHistoryEvents(db, id)
}

func (s *LocalService) AddComment(id int64, comment string) (store.Comment, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Comment{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Comment{}, err
	}
	return store.AddComment(db, id, user.ID, comment)
}

func (s *LocalService) ListComments(id int64) ([]store.Comment, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListComments(db, id)
}

func (s *LocalService) AddDependency(request DependencyRequest) (store.Dependency, error) {
	db, err := s.openDB()
	if err != nil {
		return store.Dependency{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return store.Dependency{}, err
	}
	return store.AddDependency(db, request.ProjectID, request.TicketID, request.DependsOn, user.ID)
}

func (s *LocalService) RemoveDependency(request DependencyRequest) error {
	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	return store.DeleteDependency(db, request.ProjectID, request.TicketID, request.DependsOn)
}

func (s *LocalService) ListDependencies(id int64) ([]store.Dependency, error) {
	db, err := s.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return store.ListDependencies(db, id)
}

func (s *LocalService) RequestTicket(request TicketRequest) (TicketRequestResponse, error) {
	db, err := s.openDB()
	if err != nil {
		return TicketRequestResponse{}, err
	}
	defer db.Close()
	user, err := s.localUser(db)
	if err != nil {
		return TicketRequestResponse{}, err
	}
	task, status, err := store.RequestTicket(db, store.TicketRequestParams{
		ProjectID: request.ProjectID,
		TicketID:  request.TicketID,
		TicketRef: request.TicketRef,
		Username:  user.Username,
		UserID:    user.ID,
		DryRun:    request.DryRun,
	})
	if err != nil {
		return TicketRequestResponse{}, err
	}
	response := TicketRequestResponse{Status: status}
	if status == "ASSIGNED" || status == "AVAILABLE" {
		response.Ticket = &task
	}
	return response, nil
}

func (s *LocalService) openDB() (*sql.DB, error) {
	path, err := config.ResolveDatabasePath()
	if err != nil {
		return nil, err
	}
	return store.Open(path)
}

func (s *LocalService) localUser(db *sql.DB) (store.User, error) {
	username := LocalUsername()
	if user, err := store.GetUserByUsername(db, username); err == nil {
		if user.Enabled {
			return user, nil
		}
		if err := store.SetUserEnabled(db, username, true); err != nil {
			return store.User{}, err
		}
		return store.GetUserByUsername(db, username)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, err
	}
	return store.CreateUser(db, username, "local-mode", "admin")
}

func LocalUsername() string {
	return "admin"
}
