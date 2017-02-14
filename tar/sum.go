package tar

import (
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/erikh/box/copy"
	"github.com/erikh/box/logger"
)

// SumReader sums an io.Reader
func SumReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	_, err := io.Copy(hash, reader)
	return hex.EncodeToString(hash.Sum(nil)), err
}

// SumWithCopy simultaneously sums and copies a stream.
func SumWithCopy(writer io.WriteCloser, reader io.Reader, logger *logger.Logger, fileType string) (string, error) {
	hashReader, hashWriter := io.Pipe()
	tarReader := io.TeeReader(reader, hashWriter)

	sumChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		sum, err := SumReader(hashReader)
		if err != nil {
			errChan <- err
		} else {
			sumChan <- sum
		}
	}()

	if err := copy.WithProgress(writer, tarReader, logger, fileType); err != nil {
		writer.Close()
		return "", err
	}

	writer.Close()
	hashWriter.Close()

	var sum string

	select {
	case err := <-errChan:
		return "", err
	case sum = <-sumChan:
	}

	return sum, nil
}
