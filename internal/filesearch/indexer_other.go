//go:build !windows

package filesearch

import (
	"context"
	"fmt"
)

type unsupportedIndexBuilder struct{}

func newDefaultIndexBuilder() indexBuilder {
	return unsupportedIndexBuilder{}
}

func newSharedIndexBuilder() indexBuilder {
	return unsupportedIndexBuilder{}
}

func (unsupportedIndexBuilder) Build(ctx context.Context) (IndexBuildResult, error) {
	return IndexBuildResult{}, fmt.Errorf("Ariadne 文件索引仅支持 Windows NTFS")
}
