package registry

type RestrictionType string

const (
	RestrictionSecurity RestrictionType = "security"
	RestrictionRuntime  RestrictionType = "runtime"
	RestrictionStyling  RestrictionType = "styling"
	RestrictionUX       RestrictionType = "ux"
)

type ComponentPropMetadata struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Required    bool    `json:"required"`
	Default     *string `json:"defaultValue,omitempty"`
	Description *string `json:"description,omitempty"`
}

type ComponentSlotMetadata struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Required    *bool   `json:"required,omitempty"`
}

type ComponentEventMetadata struct {
	Name        string  `json:"name"`
	Payload     *string `json:"payload,omitempty"`
	Description *string `json:"description,omitempty"`
}

type ComponentExample struct {
	Label       string  `json:"label"`
	Pug         string  `json:"pug"`
	Description *string `json:"description,omitempty"`
}

type ComponentRestriction struct {
	Type    RestrictionType `json:"type"`
	Message string          `json:"message"`
}

type ComponentInventoryItem struct {
	Name         string                  `json:"name"`
	Module       string                  `json:"module"`
	Tag          string                  `json:"tag"`
	Pack         string                  `json:"pack"`
	Props        []ComponentPropMetadata `json:"props"`
	Slots        []ComponentSlotMetadata `json:"slots"`
	Events       []ComponentEventMetadata `json:"events"`
	Examples     []ComponentExample      `json:"examples"`
	Restrictions []ComponentRestriction  `json:"restrictions"`
	Enabled      bool                    `json:"enabled"`
	Version      *string                 `json:"version,omitempty"`
}

type ComponentRegistrationPayload struct {
	Name         string                  `json:"name"`
	Module       string                  `json:"module"`
	Tag          string                  `json:"tag"`
	Pack         string                  `json:"pack"`
	Props        []ComponentPropMetadata `json:"props,omitempty"`
	Slots        []ComponentSlotMetadata `json:"slots,omitempty"`
	Events       []ComponentEventMetadata `json:"events,omitempty"`
	Examples     []ComponentExample      `json:"examples,omitempty"`
	Restrictions []ComponentRestriction  `json:"restrictions,omitempty"`
	Enabled      *bool                   `json:"enabled,omitempty"`
	Version      *string                 `json:"version,omitempty"`
}

type ComponentInventoryResponse struct {
	Version      string                  `json:"version"`
	GeneratedAt  string                  `json:"generatedAt"`
	Components   []ComponentInventoryItem `json:"components"`
}

