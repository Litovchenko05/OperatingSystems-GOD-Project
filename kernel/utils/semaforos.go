package utils

type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore crea un nuevo semáforo con un contador inicial dado
func NewSemaphore(size int) *Semaphore {
	ch := make(chan struct{}, size)
	for i := 0; i < size; i++ {
		ch <- struct{}{} // Llena el canal (ocupado)
	}
	return &Semaphore{ch: ch}
}

func (s *Semaphore) Wait() {
	// fmt.Println("Esperando signal")
	<-s.ch // Bloquea hasta que haya un permiso
	// fmt.Println("Signal recibido")
}

func (s *Semaphore) Signal() {
	// fmt.Println("Signal enviado")
	s.ch <- struct{}{} // Libera un permiso
}
