package sources

type Source interface {
	GetID()
	GetURL()
	GetTitle()
}
