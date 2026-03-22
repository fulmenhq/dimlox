package split

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fulmenhq/dimlox/internal/provider"
)

const windowsMaxPath = 260

var (
	splitRuntimeGOOS               = runtime.GOOS
	splitPreflightStderr io.Writer = os.Stderr
	invalidWindowsRunes            = `<>:"/\|?*`
)

func preflightSplitOutputs(outDir, sourceName string, meta *provider.ObjectMeta, mode Mode, opts Options, sourceCompressed bool) error {
	if err := validateSourceName(sourceName); err != nil {
		return err
	}
	longestPath, err := longestPlannedOutputPath(outDir, sourceName, meta, mode, opts, sourceCompressed)
	if err != nil {
		return err
	}
	if len(longestPath) < windowsMaxPath {
		return nil
	}
	msg := fmt.Sprintf("planned output path length %d exceeds the %d-character portability limit: %s (choose a shorter --out-dir or source filename)", len(longestPath), windowsMaxPath, longestPath)
	if splitRuntimeGOOS == "windows" {
		return fmt.Errorf("%s", msg)
	}
	if splitPreflightStderr != nil {
		_, _ = fmt.Fprintf(splitPreflightStderr, "warning: %s\n", msg)
	}
	return nil
}

func validateSourceName(sourceName string) error {
	stem, _ := textStemAndExt(sourceName)
	if strings.ContainsAny(stem, invalidWindowsRunes) {
		return fmt.Errorf("source filename stem %q contains characters that are invalid on Windows; rename the source or choose a sanitized destination name before splitting", stem)
	}
	return nil
}

func longestPlannedOutputPath(outDir, sourceName string, meta *provider.ObjectMeta, mode Mode, opts Options, sourceCompressed bool) (string, error) {
	paths := []string{manifestPath(outDir, sourceName), manifestPath(outDir, sourceName) + ".part"}
	maxIndex := estimatedMaxShardIndex(meta, mode, opts)
	compressOut := false
	if mode != ModeBinary {
		var err error
		compressOut, err = resolveOutputCompression(sourceCompressed, opts.OutFmt)
		if err != nil {
			return "", err
		}
		shardPath := textShardPath(outDir, sourceName, maxIndex, compressOut)
		paths = append(paths, shardPath, shardPath+".part")
	} else {
		shardPath := binaryShardPath(outDir, sourceName, maxIndex)
		paths = append(paths, shardPath, shardPath+".part")
	}
	longest := paths[0]
	for _, candidate := range paths[1:] {
		if len(candidate) > len(longest) {
			longest = candidate
		}
	}
	return filepath.Clean(longest), nil
}

func estimatedMaxShardIndex(meta *provider.ObjectMeta, mode Mode, opts Options) int {
	if meta == nil || meta.Size <= 0 {
		return 9999
	}
	if mode == ModeBinary && opts.Bytes > 0 {
		return maxInt(1, int((meta.Size+opts.Bytes-1)/opts.Bytes))
	}
	return maxInt(1, int(meta.Size))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
