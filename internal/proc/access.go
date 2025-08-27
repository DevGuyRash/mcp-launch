package proc

import "sync"

func (s *Supervisor) Mu() *sync.Mutex { return &s.mu }

func (s *Supervisor) ChildPID(name string) int {
    s.mu.Lock()
    defer s.mu.Unlock()
    if ch, ok := s.childs[name]; ok && ch.Cmd != nil && ch.Cmd.Process != nil {
        return ch.Cmd.Process.Pid
    }
    return 0
}
