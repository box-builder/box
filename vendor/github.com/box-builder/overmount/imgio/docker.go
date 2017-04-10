package imgio

import "github.com/docker/docker/client"

// Docker implements image i/o (overmount.Importer and overmount.Exporter)
// through docker. Note that no attempt will be made to pull the images from
// remote sources; they must exist on your client's daemon before they can be
// used by this import/export interface.
type Docker struct {
	client *client.Client
}

// NewDocker creates a new *Docker for use. If c is nil,
// `client.NewEnvClient()` will be called to initiate a new client.
func NewDocker(c *client.Client) (*Docker, error) {
	if c == nil {
		var err error
		c, err = client.NewEnvClient()
		if err != nil {
			return nil, err
		}
	}

	return &Docker{client: c}, nil
}
