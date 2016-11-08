package image

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/erikh/box/builder/config"
	bt "github.com/erikh/box/builder/tar"
)

func writeLayer(imgwriter *tar.Writer, tarFile string, tf *os.File) error {
	fi, err := os.Stat(tf.Name())
	if err != nil {
		return err
	}

	err = imgwriter.WriteHeader(&tar.Header{
		Name:     tarFile,
		Size:     fi.Size(),
		Mode:     0666,
		Typeflag: tar.TypeReg,
	})

	if err != nil {
		return err
	}

	if _, err := io.Copy(imgwriter, tf); err != nil {
		return err
	}

	return nil
}

func writeConfig(sum string, imgwriter *tar.Writer, config *config.Config) (string, error) {
	jsonFile := fmt.Sprintf("%s.json", sum)
	tarFile := fmt.Sprintf("%s/layer.tar", sum)

	manifest := []map[string]interface{}{{
		"Config": jsonFile,
		"Layers": []string{tarFile},
	}}

	image := config.ToImage([]string{"sha256:" + sum})

	content, err := json.Marshal(image)
	if err != nil {
		return "", err
	}

	err = imgwriter.WriteHeader(&tar.Header{
		Uname:      "root",
		Gname:      "root",
		Name:       jsonFile,
		Linkname:   jsonFile,
		Size:       int64(len(content)),
		Mode:       0666,
		Typeflag:   tar.TypeReg,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	})

	if err != nil {
		return "", err
	}

	if _, err := imgwriter.Write(content); err != nil {
		return "", err
	}

	imgwriter.Flush()

	content, err = json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	err = imgwriter.WriteHeader(&tar.Header{
		Name:       "manifest.json",
		Linkname:   "manifest.json",
		Uname:      "root",
		Gname:      "root",
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
		Size:       int64(len(content)),
		Mode:       0666,
		Typeflag:   tar.TypeReg,
	})

	if err != nil {
		return "", err
	}

	if _, err := imgwriter.Write(content); err != nil {
		return "", err
	}

	imgwriter.Flush()
	return tarFile, nil
}

func sum(tf *os.File, tw io.Reader) (string, error) {
	if _, err := io.Copy(tf, tw); err != nil {
		return "", err
	}

	sum, err := bt.SumFile(tf.Name())
	if err != nil {
		return "", err
	}

	if err := tf.Sync(); err != nil {
		return "", err
	}

	if _, err := tf.Seek(0, 0); err != nil {
		return "", err
	}

	return sum, nil
}

func tmpfile() (*os.File, error) {
	return ioutil.TempFile("", "image-temporary-layer")
}

// CopyToImage copies a tarred up series of files (passed in through the
// io.Reader handle) to the image where they are untarred. Returns the SHA256
// of the image created.
func CopyToImage(client *client.Client, config *config.Config, id string, size int64, tw io.Reader) (string, error) {
	out, err := tmpfile()
	if err != nil {
		return "", err
	}

	defer out.Close()
	defer os.Remove(out.Name())

	tf, err := tmpfile()
	if err != nil {
		return "", err
	}

	defer tf.Close() // second close is fine here
	defer os.Remove(tf.Name())

	sum, err := sum(tf, tw)
	if err != nil {
		return "", err
	}

	imgwriter := tar.NewWriter(out)

	tarFile, err := writeConfig(sum, imgwriter, config)
	if err != nil {
		return "", err
	}

	if err := writeLayer(imgwriter, tarFile, tf); err != nil {
		return "", err
	}

	imgwriter.Close()

	if _, err := out.Seek(0, 0); err != nil {
		return "", err
	}

	resp, err := client.ImageLoad(context.Background(), out, true)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	parts := strings.SplitN(string(content), ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("Invalid value returned from docker: %s", string(content))
	}

	return strings.TrimSpace(parts[1]), nil
}
