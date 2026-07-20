package plugins

type Plugin interface {
	Name() string
	Version() string
	Type() string
	Execute(params map[string]interface{}) (map[string]interface{}, error)
}
