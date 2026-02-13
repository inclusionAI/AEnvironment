package faas_model

const (
	FunctionInvocationTypeSync  = "RequestResponse"
	FunctionInvocationTypeAsync = "Event"
)

// InstanceStatus represents the status of an instance
type InstanceStatus string

const (
	InstanceStatusPending    InstanceStatus = "Pending"
	InstanceStatusCreating   InstanceStatus = "Creating"
	InstanceStatusRunning    InstanceStatus = "Running"
	InstanceStatusFailed     InstanceStatus = "Failed"
	InstanceStatusTerminated InstanceStatus = "Terminated"
)

type FunctionCreateOrUpdateRequest struct {
	Name        string            `json:"name"`
	PackageType string            `json:"packageType"`
	Code        *FunctionCode     `json:"code"`
	Runtime     string            `json:"runtime"`
	Labels      map[string]string `json:"labels,omitempty"`
	Envs        map[string]string `json:"envs,omitempty"`
	Handler     string            `json:"handler"`
	Description string            `json:"description"`
	Memory      int64             `json:"memory"`
	Timeout     int64             `json:"timeout"`
}

type FunctionCode struct {
	OSSURL string `json:"ossURL"`
	Image  string `json:"image"`
}

type Function struct {
	Name        string            `json:"name"`
	PackageType string            `json:"packageType"`
	Code        *FunctionCode     `json:"code"`
	Runtime     string            `json:"runtime"`
	Labels      map[string]string `json:"labels,omitempty"`
	Description string            `json:"description"`
	Memory      int64             `json:"memory"`
	Timeout     int64             `json:"timeout"`
}

type Instance struct {
	InstanceID      string            `json:"instanceID"`
	CreateTimestamp int64             `json:"createTimestamp"`
	IP              string            `json:"ip"`
	Labels          map[string]string `json:"labels"`
	Status          InstanceStatus    `json:"status"`
	TTL             string            `json:"ttl"`
}

type InstanceListResp struct {
	Instances []*Instance `json:"instances"`
}

type InstanceListRequest struct {
	Labels map[string]string `json:"labels"`
}

type APIResponse struct {
	Success      bool        `json:"success"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

type APIInstanceResponse struct {
	Success      bool     `json:"success"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
	Data         Instance `json:"data,omitempty"`
}

type APIInstanceListResponse struct {
	Success      bool              `json:"success"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
	Data         *InstanceListResp `json:"data,omitempty"`
}

type RuntimeCreateOrUpdateRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Content     *RuntimeContent   `json:"content"`
	Labels      map[string]string `json:"labels"`
}

type RuntimeContent struct {
	OssURL string `json:"ossUrl,omitempty"`
	Image  string `json:"image,omitempty"`
}

type Runtime struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Content     *RuntimeContent   `json:"content"`
	Labels      map[string]string `json:"labels"`
	Status      string            `json:"status"`
}

type RuntimeStatus string

const (
	RuntimeStatusActive    RuntimeStatus = "active"
	RuntimeStatusDeleting  RuntimeStatus = "deleting"
	RuntimeStatusPreparing RuntimeStatus = "preparing"
	RuntimeStatusError     RuntimeStatus = "error"
)
