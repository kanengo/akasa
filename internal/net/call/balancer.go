package call

type ReplicaConnection interface {
	Address() string
}

type Balancer interface {
	Add(ReplicaConnection)
	Remove(ReplicaConnection)

	Pick(CallOptions) (ReplicaConnection, bool)
}

func RoundRobin() Balancer {
	return &roundRobin{}
}

type roundRobin struct {
	connList
	next int
}

var _ Balancer = &roundRobin{}

func (rr *roundRobin) Add(connection ReplicaConnection) {
	rr.connList.Add(connection)
}

func (rr *roundRobin) Remove(connection ReplicaConnection) {
	rr.connList.Remove(connection)
}

func (rr *roundRobin) Pick(options CallOptions) (ReplicaConnection, bool) {
	if len(rr.list) == 0 {
		return nil, false
	}
	if rr.next >= len(rr.list) {
		rr.next = 0
	}
	c := rr.list[rr.next]
	rr.next += 1

	return c, true
}

// connList
type connList struct {
	list []ReplicaConnection
}

func (cl *connList) Add(connection ReplicaConnection) {
	cl.list = append(cl.list, connection)
}

func (cl *connList) Remove(connection ReplicaConnection) {
	for i, c := range cl.list {
		if c != connection {
			continue
		}
		cl.list[i] = cl.list[len(cl.list)-1]
		cl.list = cl.list[:len(cl.list)-1]
		return
	}

}
