package pubsub

import (
	"bytes"
	"encoding/gob"
	"flag"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

var (
	gen          = flag.Bool("gen", true, "If true, new test data will be generated. If false, old test data will be reused.")
	topics       = flag.Int("topics", 3, "")
	pubsPerTopic = flag.Int("pubsPerTopic", 5, "")
	subsPerTopic = flag.Int("subsPerTopic", 5, "")
	msgLen       = flag.Int("msgLen", 100, "")
)

func oldTestData(t *testing.T) (ret []string) {

	t.Helper()

	b, err := ioutil.ReadFile("test_data.txt")
	if err != nil {
		t.Fatalf("Cannot read file test_data.txt: %v", err)
	}

	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&ret); err != nil {
		t.Fatalf("Cannot decode test data: %v", err)
	}

	return
}

func newTestData(t *testing.T) (ret []string) {

	t.Helper()

	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	rand.Seed(time.Now().UnixNano())
	for i := 0; i < *topics; i++ {
		b := make([]rune, *msgLen)
		for i := range b {
			b[i] = letters[rand.Intn(len(letters))]
		}
		ret = append(ret, string(b))
	}

	buf := bytes.NewBuffer(nil)
	if err := gob.NewEncoder(buf).Encode(ret); err != nil {
		t.Fatalf("Cannot encode newly generated test data: %v", err)
	}

	if err := ioutil.WriteFile("test_data.txt", buf.Bytes(), 0666); err != nil {
		t.Fatalf("Cannot write file: %v", err)
	}

	return
}

func logEvents(t *testing.T, events chan Event) {

	t.Helper()

	l := zerolog.New(os.Stdout)
	for {
		e := <-events

		if e.EventType == EventTypeInternalServerError {
			l.Warn().Err(e.Error).
				Str("id", e.ClientID).
				Str("topic", e.Topic).
				Str("client_type", e.ClientType).
				Str("client_type", e.ClientType).
				Msg("Internal Server Error")
		}
	}
}

func TestPubsub(t *testing.T) {

	flag.Parse()

	var testData []string
	if *gen {
		testData = newTestData(t)
	} else {
		testData = oldTestData(t)
	}

}
