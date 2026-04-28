package registry

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrComponentNotFound      = errors.New("component not found")
	ErrDuplicateComponentName = errors.New("component name already exists")
	ErrComponentPayload       = errors.New("name, module, tag and pack are required")
)

type RegistryService struct {
	mu          sync.RWMutex
	components  []ComponentInventoryItem
	storagePath string
}

func NewRegistryService(storagePath string) *RegistryService {
	svc := &RegistryService{
		components:  make([]ComponentInventoryItem, len(initialComponentInventory)),
		storagePath: storagePath,
	}
	copy(svc.components, initialComponentInventory)
	_ = svc.loadFromDisk()
	return svc
}

func (s *RegistryService) List(enabledOnly bool) ComponentInventoryResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	components := make([]ComponentInventoryItem, 0, len(s.components))
	for _, item := range s.components {
		if enabledOnly && !item.Enabled {
			continue
		}
		components = append(components, item)
	}

	sort.Slice(components, func(i, j int) bool {
		return strings.ToLower(components[i].Name) < strings.ToLower(components[j].Name)
	})

	return ComponentInventoryResponse{
		Version:     CatalogVersion,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Components:  components,
	}
}

func (s *RegistryService) Get(name string) (ComponentInventoryItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.components {
		if item.Name == name {
			return item, true
		}
	}
	return ComponentInventoryItem{}, false
}

func (s *RegistryService) Register(payload ComponentRegistrationPayload) ComponentInventoryItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	enabled := false
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}

	newItem := ComponentInventoryItem{
		Name:         payload.Name,
		Module:       payload.Module,
		Tag:          payload.Tag,
		Pack:         payload.Pack,
		Props:        payload.Props,
		Slots:        payload.Slots,
		Events:       payload.Events,
		Examples:     payload.Examples,
		Restrictions: payload.Restrictions,
		Enabled:      enabled,
		Version:      payload.Version,
	}

	for idx := range s.components {
		if s.components[idx].Name == payload.Name {
			s.components[idx] = newItem
			_ = s.persistLocked()
			return newItem
		}
	}

	s.components = append(s.components, newItem)
	_ = s.persistLocked()
	return newItem
}

func (s *RegistryService) Update(name string, payload ComponentRegistrationPayload) (ComponentInventoryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	targetName := strings.TrimSpace(name)
	payloadName := strings.TrimSpace(payload.Name)
	if payloadName == "" {
		payloadName = targetName
	}
	payload.Name = payloadName

	module := strings.TrimSpace(payload.Module)
	tag := strings.TrimSpace(payload.Tag)
	pack := strings.TrimSpace(payload.Pack)
	if payloadName == "" || module == "" || tag == "" || pack == "" {
		return ComponentInventoryItem{}, ErrComponentPayload
	}
	payload.Module = module
	payload.Tag = tag
	payload.Pack = pack

	if payloadName != targetName {
		for _, item := range s.components {
			if item.Name == payloadName {
				return ComponentInventoryItem{}, ErrDuplicateComponentName
			}
		}
	}

	for idx := range s.components {
		if s.components[idx].Name == targetName {
			enabled := s.components[idx].Enabled
			if payload.Enabled != nil {
				enabled = *payload.Enabled
			}

			s.components[idx] = ComponentInventoryItem{
				Name:         payloadName,
				Module:       payload.Module,
				Tag:          payload.Tag,
				Pack:         payload.Pack,
				Props:        payload.Props,
				Slots:        payload.Slots,
				Events:       payload.Events,
				Examples:     payload.Examples,
				Restrictions: payload.Restrictions,
				Enabled:      enabled,
				Version:      payload.Version,
			}
			_ = s.persistLocked()
			return s.components[idx], nil
		}
	}
	return ComponentInventoryItem{}, ErrComponentNotFound
}

func (s *RegistryService) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	targetName := strings.TrimSpace(name)
	if targetName == "" {
		return ErrComponentNotFound
	}

	for idx := range s.components {
		if s.components[idx].Name == targetName {
			s.components = append(s.components[:idx], s.components[idx+1:]...)
			_ = s.persistLocked()
			return nil
		}
	}
	return ErrComponentNotFound
}

func (s *RegistryService) SetEnabled(name string, enabled bool) (ComponentInventoryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx := range s.components {
		if s.components[idx].Name == name {
			s.components[idx].Enabled = enabled
			_ = s.persistLocked()
			return s.components[idx], nil
		}
	}
	return ComponentInventoryItem{}, errors.New("component not found")
}

func (s *RegistryService) loadFromDisk() error {
	raw, err := os.ReadFile(s.storagePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var payload ComponentInventoryResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}

	if len(payload.Components) == 0 {
		return nil
	}
	s.components = payload.Components
	return nil
}

func (s *RegistryService) persistLocked() error {
	dir := filepath.Dir(s.storagePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	components := make([]ComponentInventoryItem, len(s.components))
	copy(components, s.components)
	payload := ComponentInventoryResponse{
		Version:     CatalogVersion,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Components:  components,
	}

	doc, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.storagePath, doc, 0o644)
}
