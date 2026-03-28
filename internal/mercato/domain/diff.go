package domain

type DiffAction int

const (
	DiffInsert DiffAction = iota
	DiffModify
	DiffDelete
	DiffRename
)

type FileDiff struct {
	Action DiffAction
	From   string
	To     string
}
