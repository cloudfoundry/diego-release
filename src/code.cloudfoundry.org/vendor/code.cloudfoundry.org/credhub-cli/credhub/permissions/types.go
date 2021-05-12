// CredHub permission types
package permissions

type Permission struct {
	Actor      string   `json:"actor"`
	Operations []string `json:"operations"`
	Path       string   `json:"path"`
	UUID       string   `json:"uuid"`
}

type V1_Permission struct {
	Actor      string   `json:"actor"`
	Operations []string `json:"operations"`
}
