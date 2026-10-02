package parquet

import "sync"

// MakeParquetMetadataCache returns an empty cache for reusable scans.
func MakeParquetMetadataCache() *ParquetMetadataCache { return &ParquetMetadataCache{} }

// ParquetMetadataCache retains immutable schema and row-group metadata for one
// exact footer. Concurrent opens may share a cache. Footer identity bytes are
// control metadata; page and temporary footer storage remain allocator-owned.
type ParquetMetadataCache struct {
	mutex   sync.Mutex
	size    int64
	footer  []byte
	columns []parquetColumn
	groups  []parquetGroup
}
