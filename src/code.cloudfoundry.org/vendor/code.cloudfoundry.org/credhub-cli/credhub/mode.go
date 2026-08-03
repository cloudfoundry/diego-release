package credhub

type Mode string

const (
	Overwrite   Mode = "overwrite"
	NoOverwrite Mode = "no-overwrite"
	Converge    Mode = "converge"
)
