package command

// Run corresponds to the `run` verb
func (i *Interpreter) Run(command string, showRun bool) error {
	i.exec.Config().TemporaryCommand([]string{"/bin/sh", "-c"}, []string{command})

	if i.globals.ShowRun == true && !showRun {
		state := i.globals.ShowRun
		i.globals.ShowRun = showRun
		defer func() { i.globals.ShowRun = state }()
	}

	return i.makeLayer(true)
}
