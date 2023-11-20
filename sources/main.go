package sources

const PagerdutyIncidentLimit = 100

type GenericEvent struct {
	GEvent interface{}
	Source string
}

func (e *GenericEvent) Id() string {
	return e.GEvent.(string)
}
