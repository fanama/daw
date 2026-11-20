package wavio

import "os"

// FinalizePartial repairs an interrupted take: it trusts the file length over
// the stale header sizes and rewrites a valid header. It returns the frame
// count that the repaired file holds.
func FinalizePartial(path string) (int, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()

	info, err := readHeader(file)
	if err != nil {
		return 0, err
	}
	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}
	dataBytes := stat.Size() - info.dataOffset
	if dataBytes < 0 {
		dataBytes = 0
	}
	blockAlign := int64(info.channels * bytesPerSample)
	frames := 0
	if blockAlign > 0 {
		frames = int(dataBytes / blockAlign)
	}
	header := buildHeader(info.channels, info.sampleRate, frames)
	if _, err := file.Seek(0, 0); err != nil {
		return 0, err
	}
	if _, err := file.Write(header); err != nil {
		return 0, err
	}
	if err := file.Sync(); err != nil {
		return 0, err
	}
	return frames, nil
}

// ReadHeader reports the channel count and rate stored in a take file.
func ReadHeader(path string) (channels, sampleRate int, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = file.Close() }()
	info, err := readHeader(file)
	if err != nil {
		return 0, 0, err
	}
	return info.channels, info.sampleRate, nil
}

// DataSize reports the byte size stored in the data chunk, which is zero for
// a take that was interrupted before its first checkpoint.
func DataSize(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()
	info, err := readHeader(file)
	if err != nil {
		return 0, err
	}
	if info.dataBytes < 0 {
		return 0, nil
	}
	return info.dataBytes, nil
}
