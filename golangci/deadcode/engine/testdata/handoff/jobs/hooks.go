package jobs

// Notice and Alarm are handed to a Notifier through a method expression
// and a method value.
type Notice struct{ Text string }

type Alarm struct{ Level int }

type Notifier interface {
	Notify(Notice)
	Alert(Alarm)
}

// Log is a Notifier a caller can drive by hand.
type Log struct{}

func (Log) Notify(Notice) {}

func (Log) Alert(Alarm) {}

func deliver[O, V any](o O, v V, send func(O, V)) { send(o, v) }

func Announce(n Notifier) { deliver(n, Notice{Text: "up"}, Notifier.Notify) }

func Broadcast(n Notifier) {
	alert := n.Alert
	alert(Alarm{Level: 3})
}

// Sample is handed to an unexported interface a caller satisfies structurally.
type Sample struct{ Value int }

type sink interface{ Put(Sample) }

// Gauge is a sink a caller can drive by hand.
type Gauge struct{}

func (Gauge) Put(Sample) {}

func Collect(s sink) { s.Put(Sample{Value: 1}) }

// Tick is handed to the func a caller assigns to OnTick.
type Tick struct{ N int }

var OnTick func(Tick)

func Step() {
	if OnTick != nil {
		OnTick(Tick{N: 1})
	}
}
