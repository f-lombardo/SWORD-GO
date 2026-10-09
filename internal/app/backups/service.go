package backups

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"sword-go/internal/app/servers"
	"sword-go/internal/app/sites"
)

type Service struct {
	store        *Store
	serverStore  *servers.Store
	siteStore    *sites.Store
	siteExecutor SiteBackupExecutor
}

func NewService(store *Store, serverStore *servers.Store, siteStore *sites.Store) *Service {
	return &Service{
		store:        store,
		serverStore:  serverStore,
		siteStore:    siteStore,
		siteExecutor: borgExecutor{},
	}
}

func (s *Service) ListDestinations(ctx context.Context) ([]Destination, error) {
	return s.store.ListDestinations(ctx)
}

func (s *Service) GetDestination(ctx context.Context, id int64) (Destination, error) {
	return s.store.GetDestinationByID(ctx, id)
}

func (s *Service) CreateDestination(ctx context.Context, input CreateDestinationInput) (Destination, error) {
	if err := validateDestinationInput(input, true); err != nil {
		return Destination{}, err
	}

	destination := Destination{
		Name:          strings.TrimSpace(input.Name),
		Type:          strings.TrimSpace(input.Type),
		Host:          strings.TrimSpace(input.Host),
		Port:          input.Port,
		Username:      strings.TrimSpace(input.Username),
		AuthMethod:    strings.TrimSpace(input.AuthMethod),
		Password:      input.Password,
		SSHPrivateKey: input.SSHPrivateKey,
		StoragePath:   strings.TrimSpace(input.StoragePath),
		Status:        "pending",
	}

	connected, errMessage := testDestinationConnection(destination)
	if connected {
		destination.Status = "connected"
		now := time.Now().UTC()
		destination.LastConnectedAt = &now
	} else {
		destination.Status = "error"
	}

	created, err := s.store.CreateDestination(ctx, destination)
	if err != nil {
		return Destination{}, err
	}

	if errMessage != "" {
		return created, fmt.Errorf("connection failed: %s", errMessage)
	}

	return created, nil
}

func (s *Service) UpdateDestination(ctx context.Context, id int64, input UpdateDestinationInput) (Destination, error) {
	existing, err := s.store.GetDestinationByID(ctx, id)
	if err != nil {
		return Destination{}, err
	}

	if err = validateUpdateDestinationInput(input); err != nil {
		return Destination{}, err
	}

	if strings.TrimSpace(input.Password) == "" && existing.AuthMethod == "password" {
		input.Password = existing.Password
	}
	if strings.TrimSpace(input.SSHPrivateKey) == "" && existing.AuthMethod == "ssh_key" {
		input.SSHPrivateKey = existing.SSHPrivateKey
	}

	existing.Name = strings.TrimSpace(input.Name)
	existing.Type = strings.TrimSpace(input.Type)
	existing.Host = strings.TrimSpace(input.Host)
	existing.Port = input.Port
	existing.Username = strings.TrimSpace(input.Username)
	existing.AuthMethod = strings.TrimSpace(input.AuthMethod)
	existing.Password = input.Password
	existing.SSHPrivateKey = input.SSHPrivateKey
	existing.StoragePath = strings.TrimSpace(input.StoragePath)

	connected, errMessage := testDestinationConnection(existing)
	if connected {
		existing.Status = "connected"
		now := time.Now().UTC()
		existing.LastConnectedAt = &now
	} else {
		existing.Status = "error"
	}

	if err = s.store.UpdateDestination(ctx, existing); err != nil {
		return Destination{}, err
	}

	updated, err := s.store.GetDestinationByID(ctx, id)
	if err != nil {
		return Destination{}, err
	}
	if errMessage != "" {
		return updated, fmt.Errorf("connection failed: %s", errMessage)
	}
	return updated, nil
}

func (s *Service) DeleteDestination(ctx context.Context, id int64) error {
	return s.store.DeleteDestination(ctx, id)
}

func (s *Service) ListSchedules(ctx context.Context) ([]Schedule, error) {
	return s.store.ListSchedules(ctx)
}

func (s *Service) ListSchedulesByServer(ctx context.Context, serverID int64) ([]Schedule, error) {
	return s.store.ListSchedulesByServer(ctx, serverID)
}

func (s *Service) CreateSchedule(ctx context.Context, input CreateScheduleInput) (Schedule, error) {
	if err := validateScheduleInput(input); err != nil {
		return Schedule{}, err
	}

	if _, err := s.serverStore.GetByID(ctx, input.ServerID); err != nil {
		return Schedule{}, errors.New("server not found")
	}
	if _, err := s.store.GetDestinationByID(ctx, input.BackupDestinationID); err != nil {
		return Schedule{}, errors.New("backup destination is invalid")
	}

	return s.store.CreateSchedule(ctx, Schedule{
		ServerID:            input.ServerID,
		BackupDestinationID: input.BackupDestinationID,
		Frequency:           input.Frequency,
		Time:                input.Time,
		DayOfWeek:           input.DayOfWeek,
		DayOfMonth:          input.DayOfMonth,
		RetentionCount:      input.RetentionCount,
		IsEnabled:           true,
	})
}

func (s *Service) DeleteSchedule(ctx context.Context, serverID int64, scheduleID int64) error {
	schedule, err := s.store.GetScheduleByID(ctx, scheduleID)
	if err != nil {
		return err
	}
	if schedule.ServerID != serverID {
		return ErrScheduleNotFound
	}
	return s.store.DeleteSchedule(ctx, scheduleID)
}

func (s *Service) RunScheduleNow(ctx context.Context, serverID int64, scheduleID int64) error {
	schedule, err := s.store.GetScheduleByID(ctx, scheduleID)
	if err != nil {
		return err
	}
	if schedule.ServerID != serverID {
		return ErrScheduleNotFound
	}

	go s.executeSchedule(schedule)
	return nil
}

func (s *Service) ListRunsByServer(ctx context.Context, serverID int64, limit int) ([]Run, error) {
	return s.store.ListRunsByServer(ctx, serverID, limit)
}

func (s *Service) DispatchDueBackups(ctx context.Context, now time.Time) (int, error) {
	schedules, err := s.store.ListSchedules(ctx)
	if err != nil {
		return 0, err
	}

	dispatched := 0
	for _, schedule := range schedules {
		if !schedule.IsEnabled {
			continue
		}

		server, err := s.serverStore.GetByID(ctx, schedule.ServerID)
		if err != nil {
			continue
		}

		if !isDue(schedule, server, now) {
			continue
		}

		running, err := s.store.HasRunningRunForSchedule(ctx, schedule.ID)
		if err != nil || running {
			continue
		}

		alreadyRan, err := alreadyRanInCurrentPeriod(ctx, s.store, schedule, server, now)
		if err != nil || alreadyRan {
			continue
		}

		dispatched++
		go s.executeSchedule(schedule)
	}

	return dispatched, nil
}

func (s *Service) executeSchedule(schedule Schedule) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	destination, err := s.store.GetDestinationByID(ctx, schedule.BackupDestinationID)
	if err != nil {
		return
	}
	server, err := s.serverStore.GetByID(ctx, schedule.ServerID)
	if err != nil {
		return
	}

	installedSites, err := s.siteStore.ListInstalledByServer(ctx, schedule.ServerID)
	if err != nil {
		return
	}

	for _, site := range installedSites {
		startedAt := time.Now().UTC()
		siteID := site.ID
		run, err := s.store.CreateRun(ctx, Run{
			BackupScheduleID:    schedule.ID,
			ServerID:            schedule.ServerID,
			SiteID:              &siteID,
			BackupDestinationID: schedule.BackupDestinationID,
			Status:              "running",
			StartedAt:           &startedAt,
		})
		if err != nil {
			continue
		}

		executionResult, executionErr := s.siteExecutor.ExecuteSiteBackup(ctx, schedule, destination, server, site)

		duration := int64(time.Since(startedAt).Seconds())
		if duration < 1 {
			duration = 1
		}
		run.DurationSeconds = &duration
		completedAt := time.Now().UTC()
		run.CompletedAt = &completedAt

		if executionErr != nil {
			run.Status = "failed"
			run.Output = executionResult.Output
			if run.Output == "" {
				run.Output = executionErr.Error()
			} else {
				run.Output += "\n[error]\n" + executionErr.Error()
			}
			_ = s.store.UpdateRun(ctx, run)
			continue
		}

		run.Status = "completed"
		run.Output = executionResult.Output
		run.ArchiveName = executionResult.ArchiveName
		run.SizeBytes = executionResult.SizeBytes
		_ = s.store.UpdateRun(ctx, run)
	}
}

func validateDestinationInput(input CreateDestinationInput, requireSecrets bool) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("give your backup destination a name")
	}
	if strings.TrimSpace(input.Type) != "borg" {
		return errors.New("type must be borg")
	}
	if strings.TrimSpace(input.Host) == "" {
		return errors.New("a host address is required")
	}
	if input.Port < 1 || input.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if strings.TrimSpace(input.Username) == "" {
		return errors.New("a username is required")
	}
	if strings.TrimSpace(input.AuthMethod) != "password" && strings.TrimSpace(input.AuthMethod) != "ssh_key" {
		return errors.New("auth_method must be password or ssh_key")
	}
	if strings.TrimSpace(input.StoragePath) == "" {
		return errors.New("a storage path is required")
	}
	if requireSecrets {
		if input.AuthMethod == "password" && strings.TrimSpace(input.Password) == "" {
			return errors.New("a password is required when using password authentication")
		}
		if input.AuthMethod == "ssh_key" && strings.TrimSpace(input.SSHPrivateKey) == "" {
			return errors.New("an SSH private key is required when using SSH key authentication")
		}
	}
	return nil
}

func validateUpdateDestinationInput(input UpdateDestinationInput) error {
	return validateDestinationInput(CreateDestinationInput{
		Name:          input.Name,
		Type:          input.Type,
		Host:          input.Host,
		Port:          input.Port,
		Username:      input.Username,
		AuthMethod:    input.AuthMethod,
		Password:      input.Password,
		SSHPrivateKey: input.SSHPrivateKey,
		StoragePath:   input.StoragePath,
	}, false)
}

func validateScheduleInput(input CreateScheduleInput) error {
	if input.ServerID < 1 {
		return errors.New("server is required")
	}
	if input.BackupDestinationID < 1 {
		return errors.New("backup destination is required")
	}
	if input.Frequency != "daily" && input.Frequency != "weekly" && input.Frequency != "monthly" {
		return errors.New("frequency must be daily, weekly or monthly")
	}
	if !regexp.MustCompile(`^\d{2}:\d{2}$`).MatchString(input.Time) {
		return errors.New("time must be in HH:MM format")
	}
	if input.Frequency == "weekly" {
		if input.DayOfWeek == nil || *input.DayOfWeek < 0 || *input.DayOfWeek > 6 {
			return errors.New("day_of_week must be between 0 and 6")
		}
	}
	if input.Frequency == "monthly" {
		if input.DayOfMonth == nil || *input.DayOfMonth < 1 || *input.DayOfMonth > 28 {
			return errors.New("day_of_month must be between 1 and 28")
		}
	}
	if input.RetentionCount < 1 || input.RetentionCount > 365 {
		return errors.New("retention_count must be between 1 and 365")
	}
	return nil
}

func isDue(schedule Schedule, server servers.Server, now time.Time) bool {
	location, err := time.LoadLocation(server.Timezone)
	if err != nil {
		location = time.UTC
	}
	localNow := now.In(location)
	if localNow.Format("15:04") != schedule.Time {
		return false
	}
	switch schedule.Frequency {
	case "daily":
		return true
	case "weekly":
		return schedule.DayOfWeek != nil && int(localNow.Weekday()) == *schedule.DayOfWeek
	case "monthly":
		return schedule.DayOfMonth != nil && localNow.Day() == *schedule.DayOfMonth
	default:
		return false
	}
}

func alreadyRanInCurrentPeriod(ctx context.Context, store *Store, schedule Schedule, server servers.Server, now time.Time) (bool, error) {
	location, err := time.LoadLocation(server.Timezone)
	if err != nil {
		location = time.UTC
	}
	localNow := now.In(location)

	var startLocal time.Time
	var endLocal time.Time
	switch schedule.Frequency {
	case "daily":
		startLocal = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
		endLocal = startLocal.Add(24*time.Hour - time.Nanosecond)
	case "weekly":
		weekdayOffset := int(localNow.Weekday())
		startLocal = time.Date(localNow.Year(), localNow.Month(), localNow.Day()-weekdayOffset, 0, 0, 0, 0, location)
		endLocal = startLocal.Add(7*24*time.Hour - time.Nanosecond)
	case "monthly":
		startLocal = time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, location)
		endLocal = startLocal.AddDate(0, 1, 0).Add(-time.Nanosecond)
	default:
		return false, nil
	}
	return store.HasCompletedRunBetween(ctx, schedule.ID, startLocal.UTC(), endLocal.UTC())
}

func testDestinationConnection(destination Destination) (bool, string) {
	authMethods := make([]ssh.AuthMethod, 0, 1)
	switch destination.AuthMethod {
	case "password":
		authMethods = append(authMethods, ssh.Password(destination.Password))
	case "ssh_key":
		signer, err := parsePrivateKey(destination.SSHPrivateKey)
		if err != nil {
			return false, err.Error()
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	default:
		return false, "unsupported auth method"
	}

	config := &ssh.ClientConfig{
		User:            destination.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	address := destination.Host + ":" + strconv.Itoa(destination.Port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return false, err.Error()
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return false, err.Error()
	}
	defer session.Close()

	command := "mkdir -p " + shellEscape(destination.StoragePath)
	if err = session.Run(command); err != nil {
		return false, err.Error()
	}

	return true, ""
}

func parsePrivateKey(privateKey string) (ssh.Signer, error) {
	key := strings.TrimSpace(privateKey)
	if key == "" {
		return nil, errors.New("empty private key")
	}
	return ssh.ParsePrivateKey([]byte(key))
}

func shellEscape(value string) string {
	escaped := strings.ReplaceAll(value, `'`, `'\''`)
	return "'" + escaped + "'"
}
