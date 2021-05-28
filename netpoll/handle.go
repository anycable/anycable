package netpoll

// Desc is a network connection within netpoll descriptor.
// It's methods are not goroutine safe.
type Desc struct {
	file  int
	event Event
}

// NewDesc creates descriptor from custom fd.
func NewDesc(fd uintptr, ev Event) *Desc {
	return &Desc{0, ev}
}

// Close closes underlying file.
func (h *Desc) Close() error {
	return nil
}

func (h *Desc) fd() int {
	return h.file
}

// Must is a helper that wraps a call to a function returning (*Desc, error).
// It panics if the error is non-nil and returns desc if not.
// It is intended for use in short Desc initializations.
func Must(desc *Desc, err error) *Desc {
	if err != nil {
		panic(err)
	}
	return desc
}

// HandleRead creates read descriptor for further use in Poller methods.
// It is the same as Handle(fd, EventRead|EventEdgeTriggered).
func HandleRead(fd int) (*Desc, error) {
	return Handle(fd, EventRead|EventEdgeTriggered)
}

// HandleReadOnce creates read descriptor for further use in Poller methods.
// It is the same as Handle(fd, EventRead|EventOneShot).
func HandleReadOnce(fd int) (*Desc, error) {
	return Handle(fd, EventRead|EventOneShot)
}

// HandleWrite creates write descriptor for further use in Poller methods.
// It is the same as Handle(fd, EventWrite|EventEdgeTriggered).
func HandleWrite(fd int) (*Desc, error) {
	return Handle(fd, EventWrite|EventEdgeTriggered)
}

// HandleWriteOnce creates write descriptor for further use in Poller methods.
// It is the same as Handle(fd, EventWrite|EventOneShot).
func HandleWriteOnce(fd int) (*Desc, error) {
	return Handle(fd, EventWrite|EventOneShot)
}

// HandleReadWrite creates read and write descriptor for further use in Poller
// methods.
// It is the same as Handle(fd, EventRead|EventWrite|EventEdgeTriggered).
func HandleReadWrite(fd int) (*Desc, error) {
	return Handle(fd, EventRead|EventWrite|EventEdgeTriggered)
}

// Handle creates new Desc with given conn and event.
// Returned descriptor could be used as argument to Start(), Resume() and
// Stop() methods of some Poller implementation.
func Handle(fd int, event Event) (*Desc, error) {
	desc, err := handle(fd, event)
	if err != nil {
		return nil, err
	}

	return desc, nil
}

func handle(fd int, event Event) (*Desc, error) {
	return &Desc{
		file:  fd,
		event: event,
	}, nil
}
