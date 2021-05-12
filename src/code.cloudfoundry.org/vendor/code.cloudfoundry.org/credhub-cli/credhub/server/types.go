// CredHub server types
package server

type Info struct {
	App struct {
		Name    string
		Version string
	}
	AuthServer struct {
		URL string
	} `json:"auth-server"`
}

type VersionData struct {
	Version string
}
