package uaa

func contains(slice []string, toFind string) bool {
	for _, a := range slice {
		if a == toFind {
			return true
		}
	}
	return false
}
