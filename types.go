package main

type Secrets map[string]string

type ClusterTemplateCloneRequest struct {
	TemplateRepository string  `json:"templateRepository,omitempty"`
	Secrets            Secrets `json:"secrets,omitempty"`
	TargetOrg          string  `json:"targetOrg,omitempty"`
	TargetRepo         string  `json:"targetRepo,omitempty"`
	GitHubUser         string  `json:"gitHubUser,omitempty"`
	GitHubToken        string  `json:"gitHubToken,omitempty"`
}

type ProfileApplyRequest struct {
	TargetOrg   string   `json:"targetOrg,omitempty"`
	TargetRepo  string   `json:"targetRepo,omitempty"`
	GitHubUser  string   `json:"gitHubUser,omitempty"`
	GitHubToken string   `json:"gitHubToken,omitempty"`
	Profiles    []string `json:"profiles,omitempty"`
}

type ClusterStateApplyRequest struct {
	TargetOrg    string `json:"targetOrg,omitempty"`
	TargetRepo   string `json:"targetRepo,omitempty"`
	GitHubUser   string `json:"gitHubUser,omitempty"`
	GitHubToken  string `json:"gitHubToken,omitempty"`
	ClusterState string `json:"clusterState,omitempty"`
}

type StatusRequest struct {
	TargetOrg   string `json:"targetOrg,omitempty"`
	TargetRepo  string `json:"targetRepo,omitempty"`
	GitHubUser  string `json:"gitHubUser,omitempty"`
	GitHubToken string `json:"gitHubToken,omitempty"`
}

type RunStatus struct {
	Status     *string `json:"status,omitempty"`
	Message    *string `json:"message,omitempty"`
	Conclusion *string `json:"conclusion,omitempty"`
}

type ListClusterRequest struct {
	GitHubUser  string `json:"gitHubUser,omitempty"`
	GitHubToken string `json:"gitHubToken,omitempty"`
}

type ClusterStatus struct {
	Name      string      `json:"name,omitempty"`
	Status    string      `json:"status,omitempty"`
	Conclusion string     `json:"conclusion,omitempty"`
	Link      string      `json:"link,omitempty"`
	RunStatus []RunStatus `json:"runStatus,omitempty"`
}

