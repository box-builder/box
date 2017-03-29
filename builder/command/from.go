package command

import "sync"

var (
	pulls     = map[string]chan struct{}{}
	pullMutex = new(sync.Mutex)
)

// ResetPulls is a function to facilitate testing of the coordinated pull functionality.
func ResetPulls() {
	pulls = map[string]chan struct{}{}
}

// From corresponds to the `from` verb.
func (i *Interpreter) From(image string) error {
	if image == "scratch" || image == "" {
		return i.makeLayer(false)
	}

	var (
		pullChan chan struct{}
		pulling  bool
	)

	pullMutex.Lock()
	if pulls[image] == nil {
		pullChan = make(chan struct{})
		pulls[image] = pullChan
	} else {
		pulling = true
		pullChan = pulls[image]
	}
	pullMutex.Unlock()

	var (
		id  string
		err error
	)

	if pulling {
		<-pullChan
		id, err = i.exec.Layers().Lookup(image)
		if err != nil {
			return err
		}
	} else {
		id, err = i.exec.Layers().Fetch(i.exec.Config(), image)
		close(pullChan)
		if err != nil {
			return err
		}
	}

	i.exec.Config().Image = id

	return nil
}
