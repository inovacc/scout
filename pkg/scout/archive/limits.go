package archive

// Archive extraction safety limits. A compressed archive can expand to far more
// than its on-disk size ("decompression bomb"), pack a huge number of tiny
// entries, or carry crafted headers that drive a parser out of bounds. These
// caps bound each axis so a hostile archive yields a clean error instead of
// memory/disk exhaustion or a panic.
const (
	maxRPMUncompressed = 512 << 20 // total cpio payload after gunzip
	maxCPIOName        = 4096      // a single cpio entry name (bytes)
	maxEntryBytes      = 512 << 20 // a single extracted file
	maxTotalBytes      = 2 << 30   // total extracted across all entries
	maxEntries         = 50_000    // entry-count ceiling
	maxDebMemberSize   = 1 << 30   // a single ar member (deb)
)
