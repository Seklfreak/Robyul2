package channels

type broadcast struct {
    c chan broadcast
    v []byte
}

type Broadcaster struct {
    ListenChannel chan chan (chan broadcast)
    SendChannel   chan <- []byte
}

type Receiver struct {
    C chan broadcast
}

func NewBroadcaster() Broadcaster {
    listenc := make(chan (chan (chan broadcast)))
    sendc := make(chan []byte)
    go func() {
        currc := make(chan broadcast, 1)
        for {
            select {
            case v := <-sendc:
                if v == nil {
                    currc <- broadcast{}
                    return;
                }
                c := make(chan broadcast, 1)
                b := broadcast{c: c, v: v}
                currc <- b
                currc = c
            case r := <-listenc:
                r <- currc
            }
        }
    }()

    return Broadcaster{
        ListenChannel: listenc,
        SendChannel: sendc,
    }
}

func (b Broadcaster) Listen() Receiver {
    c := make(chan chan broadcast, 0)
    b.ListenChannel <- c
    return Receiver{<-c}
}

func (b Broadcaster) Write(v []byte) {
    b.SendChannel <- v
}

func (r *Receiver) Read() interface{} {
    b := <-r.C
    v := b.v
    r.C <- b
    r.C = b.c

    return v
}
