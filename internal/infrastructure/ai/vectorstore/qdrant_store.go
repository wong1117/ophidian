package vectorstore

type QdrantStore struct {
	host       string
	port       int
	collection string
}

func NewQdrantStore(host string, port int, collection string) *QdrantStore {
	return &QdrantStore{
		host:       host,
		port:       port,
		collection: collection,
	}
}

func (s *QdrantStore) Search(query []float32, limit int) ([]SearchResult, error) {
	return nil, nil
}

type SearchResult struct {
	ID       string
	Score    float64
	Payload  map[string]interface{}
}
