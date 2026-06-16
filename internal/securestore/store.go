package securestore

const (
	TargetOpenAIAPIKey    = "Ariadne/OpenAI/APIKey"
	TargetEmbeddingAPIKey = "Ariadne/Embedding/APIKey"
	TargetMilvusToken     = "Ariadne/Milvus/Token"
)

type Store interface {
	Available() bool
	Backend() string
	Read(target string) (string, bool, error)
	Write(target string, secret string) error
	Delete(target string) error
}

type DefaultStore struct{}

func (DefaultStore) Available() bool {
	return defaultAvailable()
}

func (DefaultStore) Backend() string {
	return defaultBackend()
}

func (DefaultStore) Read(target string) (string, bool, error) {
	return Read(target)
}

func (DefaultStore) Write(target string, secret string) error {
	return Write(target, secret)
}

func (DefaultStore) Delete(target string) error {
	return Delete(target)
}
