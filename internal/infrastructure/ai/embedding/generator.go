package embedding

type Generator struct {
	model string
}

func NewGenerator(model string) *Generator {
	return &Generator{model: model}
}

func (g *Generator) Generate(text string) ([]float32, error) {
	return nil, nil
}
