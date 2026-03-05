package libtickethttp

import (
	"github.com/simonski/ticket/internal/client"
	"github.com/simonski/ticket/internal/config"
	"github.com/simonski/ticket/internal/store"
	"github.com/simonski/ticket/libticket"
)

type Service struct {
	client *client.Client
}

func New(cfg config.Config) *Service {
	return &Service{client: client.New(cfg)}
}

func (s *Service) Status() (libticket.StatusResponse, error) {
	status, err := s.client.Status()
	if err != nil {
		return libticket.StatusResponse{}, err
	}
	return libticket.StatusResponse(status), nil
}

func (s *Service) Register(username, password string) (store.User, error) {
	return s.client.Register(username, password)
}

func (s *Service) Login(username, password string) (store.User, string, error) {
	response, err := s.client.Login(username, password)
	if err != nil {
		return store.User{}, "", err
	}
	return response.User, response.Token, nil
}

func (s *Service) Logout() error {
	return s.client.Logout()
}

func (s *Service) Count(projectID *int64) (libticket.CountSummary, error) {
	return s.client.Count(projectID)
}

func (s *Service) CreateUser(username, password string) (store.User, error) {
	return s.client.CreateUser(username, password)
}

func (s *Service) SetUserEnabled(username string, enabled bool) error {
	return s.client.SetUserEnabled(username, enabled)
}

func (s *Service) ListUsers() ([]store.User, error) {
	return s.client.ListUsers()
}

func (s *Service) DeleteUser(username string) error {
	return s.client.DeleteUser(username)
}

func (s *Service) CreateProject(request libticket.ProjectCreateRequest) (store.Project, error) {
	return s.client.CreateProject(client.ProjectCreateRequest(request))
}

func (s *Service) ListProjects() ([]store.Project, error) {
	return s.client.ListProjects()
}

func (s *Service) GetProject(id string) (store.Project, error) {
	return s.client.GetProject(id)
}

func (s *Service) UpdateProject(id int64, request libticket.ProjectUpdateRequest) (store.Project, error) {
	return s.client.UpdateProject(id, client.ProjectUpdateRequest(request))
}

func (s *Service) SetProjectEnabled(id int64, enabled bool) (store.Project, error) {
	return s.client.SetProjectEnabled(id, enabled)
}

func (s *Service) CreateTicket(request libticket.TicketCreateRequest) (store.Ticket, error) {
	return s.client.CreateTicket(client.TicketCreateRequest(request))
}

func (s *Service) ListTickets(projectID int64) ([]store.Ticket, error) {
	return s.client.ListTickets(projectID)
}

func (s *Service) ListTicketsFiltered(projectID int64, taskType, stage, state, status, search, assignee string, limit int) ([]store.Ticket, error) {
	return s.client.ListTicketsFiltered(projectID, taskType, stage, state, status, search, assignee, limit)
}

func (s *Service) UpdateTicket(id int64, request libticket.TicketUpdateRequest) (store.Ticket, error) {
	return s.client.UpdateTicket(id, client.TicketUpdateRequest(request))
}

func (s *Service) DeleteTicket(id int64) error {
	return s.client.DeleteTicket(id)
}

func (s *Service) SetTicketParent(id, parentID int64) (store.Ticket, error) {
	return s.client.SetTicketParent(id, parentID)
}

func (s *Service) UnsetTicketParent(id int64) (store.Ticket, error) {
	return s.client.UnsetTicketParent(id)
}

func (s *Service) GetTicketByID(id int64) (store.Ticket, error) {
	return s.client.GetTicketByID(id)
}

func (s *Service) GetTicket(ref string) (store.Ticket, error) {
	return s.client.GetTicket(ref)
}

func (s *Service) CloneTicket(id int64) (store.Ticket, error) {
	return s.client.CloneTicket(id)
}

func (s *Service) ListHistory(id int64) ([]store.HistoryEvent, error) {
	return s.client.ListHistory(id)
}

func (s *Service) AddComment(id int64, comment string) (store.Comment, error) {
	return s.client.AddComment(id, comment)
}

func (s *Service) ListComments(id int64) ([]store.Comment, error) {
	return s.client.ListComments(id)
}

func (s *Service) SetTicketHealth(id int64, score int) (store.Ticket, error) {
	return s.client.SetTicketHealth(id, score)
}

func (s *Service) AddDependency(request libticket.DependencyRequest) (store.Dependency, error) {
	return s.client.AddDependency(client.DependencyRequest(request))
}

func (s *Service) RemoveDependency(request libticket.DependencyRequest) error {
	return s.client.RemoveDependency(client.DependencyRequest(request))
}

func (s *Service) ListDependencies(id int64) ([]store.Dependency, error) {
	return s.client.ListDependencies(id)
}

func (s *Service) RequestTicket(request libticket.TicketRequest) (libticket.TicketRequestResponse, error) {
	response, err := s.client.RequestTicket(client.TicketRequest(request))
	if err != nil {
		return libticket.TicketRequestResponse{}, err
	}
	return libticket.TicketRequestResponse(response), nil
}
