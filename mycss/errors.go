package mycss

import "fmt"

type ErrPodNotFound struct {
	PodID
}

func (e ErrPodNotFound) Error() string {
	return fmt.Sprintf("pod %d not found", e.PodID)
}
