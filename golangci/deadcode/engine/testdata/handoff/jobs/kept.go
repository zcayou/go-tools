package jobs

// Probe is handed only to a closed interface the module implements.
type Probe struct {
	Host string
	Port int
}

type checker interface{ check(Probe) }

type tcp struct{}

func (tcp) check(p Probe) { _ = p.Host }

func Check(p Probe) {
	var c checker = tcp{}
	c.check(p)
}

// Verdict is what a caller's Classifier returns: the module overwrites two
// of its fields and reads the third.
type Verdict struct {
	Phase string
	Code  int
	Note  string
}

type Classifier interface{ Classify(error) Verdict }

func Classify(c Classifier, err error) string {
	v := c.Classify(err)
	v.Phase = "classified"
	v.Code = 1
	return v.Note
}

// Item is stored and handed back, which reads none of its fields.
type Item struct {
	Key   string
	Value int
}

type Cache struct{ items map[string]Item }

func (c *Cache) Put(it Item) { c.items[it.Key] = it }

func (c *Cache) Get(key string) Item { return c.items[key] }

// Defaults is a variable a caller assigns and the module reads.
var Defaults = Settings{Limit: 10}

type Settings struct {
	Limit  int
	Region string
}

func Limit() int { return Defaults.Limit }

func NewCache() *Cache { return &Cache{items: map[string]Item{}} }
