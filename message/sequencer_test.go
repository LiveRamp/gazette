package message

import (
	"bytes"
	"io"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "go.gazette.dev/core/broker/protocol"
)

func TestSequencerRingAddAndEvict(t *testing.T) {
	var (
		seq      = NewSequencer(nil, 5)
		generate = newTestMsgGenerator()
		A, B     = NewProducerID(), NewProducerID()
		jpA      = JournalProducer{Journal: "test/journal", Producer: A}
		jpB      = JournalProducer{Journal: "test/journal", Producer: B}
		e1       = generate(A, 1, Flag_CONTINUE_TXN)
		e2       = generate(B, 2, Flag_CONTINUE_TXN)
		e3       = generate(A, 3, Flag_CONTINUE_TXN)
		e4       = generate(A, 4, Flag_CONTINUE_TXN)
		e5       = generate(B, 5, Flag_CONTINUE_TXN)
		e6       = generate(B, 6, Flag_CONTINUE_TXN)
		e7       = generate(B, 7, Flag_CONTINUE_TXN)
		e8       = generate(B, 8, Flag_CONTINUE_TXN)
		e9       = generate(B, 9, Flag_CONTINUE_TXN)
		e10      = generate(B, 10, Flag_CONTINUE_TXN)
		e11      = generate(B, 11, Flag_CONTINUE_TXN)
	)
	// Initial ring is empty.
	assert.Equal(t, []Envelope{}, seq.ring)
	assert.Equal(t, []int{}, seq.next)
	assert.Equal(t, 0, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{}, seq.partials)

	seq.QueueUncommitted(e1) // A.
	assert.Equal(t, []Envelope{e1}, seq.ring)
	assert.Equal(t, []int{-1}, seq.next)
	assert.Equal(t, 1, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: 0, ringStop: 0},
	}, seq.partials)

	seq.QueueUncommitted(e2) // B.
	assert.Equal(t, []Envelope{e1, e2}, seq.ring)
	assert.Equal(t, []int{-1, -1}, seq.next)
	assert.Equal(t, 2, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: 0, ringStop: 0},
		jpB: {begin: e2.Begin, ringStart: 1, ringStop: 1},
	}, seq.partials)

	seq.QueueUncommitted(e3) // A.
	assert.Equal(t, []Envelope{e1, e2, e3}, seq.ring)
	assert.Equal(t, []int{2, -1, -1}, seq.next) // e1 => e3.
	assert.Equal(t, 3, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: 0, ringStop: 2},
		jpB: {begin: e2.Begin, ringStart: 1, ringStop: 1},
	}, seq.partials)

	seq.QueueUncommitted(e4) // A.
	assert.Equal(t, []Envelope{e1, e2, e3, e4}, seq.ring)
	assert.Equal(t, []int{2, -1, 3, -1}, seq.next) // e3 => e4.
	assert.Equal(t, 4, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: 0, ringStop: 3},
		jpB: {begin: e2.Begin, ringStart: 1, ringStop: 1},
	}, seq.partials)

	seq.QueueUncommitted(e5) // B.
	assert.Equal(t, []Envelope{e1, e2, e3, e4, e5}, seq.ring)
	assert.Equal(t, []int{2, 4, 3, -1, -1}, seq.next) // e2 => e5.
	assert.Equal(t, 0, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: 0, ringStop: 3},
		jpB: {begin: e2.Begin, ringStart: 1, ringStop: 4},
	}, seq.partials)

	seq.QueueUncommitted(e6) // B.
	assert.Equal(t, []Envelope{e6, e2, e3, e4, e5}, seq.ring)
	assert.Equal(t, []int{-1, 4, 3, -1, 0}, seq.next) // e5 => e6.
	assert.Equal(t, 1, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: 2, ringStop: 3},
		jpB: {begin: e2.Begin, ringStart: 1, ringStop: 0},
	}, seq.partials)

	seq.QueueUncommitted(e7) // B.
	seq.QueueUncommitted(e8) // B.
	assert.Equal(t, []Envelope{e6, e7, e8, e4, e5}, seq.ring)
	assert.Equal(t, []int{1, 2, -1, -1, 0}, seq.next)
	assert.Equal(t, 3, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: 3, ringStop: 3},
		jpB: {begin: e2.Begin, ringStart: 4, ringStop: 2},
	}, seq.partials)

	seq.QueueUncommitted(e9) // B. Evicts final A entry.
	assert.Equal(t, []Envelope{e6, e7, e8, e9, e5}, seq.ring)
	assert.Equal(t, []int{1, 2, 3, -1, 0}, seq.next)
	assert.Equal(t, 4, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		// A's begin is still tracked, but it's no longer in the ring.
		jpA: {begin: e1.Begin, ringStart: -1, ringStop: -1},
		jpB: {begin: e2.Begin, ringStart: 4, ringStop: 3},
	}, seq.partials)

	seq.QueueUncommitted(e10) // B.
	assert.Equal(t, []Envelope{e6, e7, e8, e9, e10}, seq.ring)
	assert.Equal(t, []int{1, 2, 3, 4, -1}, seq.next)
	assert.Equal(t, 0, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: -1, ringStop: -1}, // Unchanged.
		jpB: {begin: e2.Begin, ringStart: 0, ringStop: 4},
	}, seq.partials)

	seq.QueueUncommitted(e11) // B.
	assert.Equal(t, []Envelope{e11, e7, e8, e9, e10}, seq.ring)
	assert.Equal(t, []int{-1, 2, 3, 4, 0}, seq.next)
	assert.Equal(t, 1, seq.head)
	assert.Equal(t, map[JournalProducer]partialSeq{
		jpA: {begin: e1.Begin, ringStart: -1, ringStop: -1}, // Unchanged.
		jpB: {begin: e2.Begin, ringStart: 1, ringStop: 0},
	}, seq.partials)
}

func TestSequencerTxnSequenceCases(t *testing.T) {
	var (
		generate = newTestMsgGenerator()
		seq      = NewSequencer(nil, 3)
		A, B     = NewProducerID(), NewProducerID()
	)

	// Case: Sequence with internal duplicates served from the ring.
	var (
		a1    = generate(A, 1, Flag_CONTINUE_TXN)
		a2    = generate(A, 2, Flag_CONTINUE_TXN)
		a1Dup = generate(A, 1, Flag_CONTINUE_TXN)
		a2Dup = generate(A, 2, Flag_CONTINUE_TXN)
		a3ACK = generate(A, 3, Flag_ACK_TXN)
	)
	queue(seq, a1, a2, a1Dup, a2Dup, a3ACK)
	expectDeque(t, seq, a1, a2, a3ACK)

	// Case: ACK w/o preceding CONTINUE. Unusual but allowed.
	var a4ACK = generate(A, 4, Flag_ACK_TXN)
	queue(seq, a4ACK)
	expectDeque(t, seq, a4ACK)

	// Case: Partial ACK of preceding messages.
	var (
		a5      = generate(A, 5, Flag_CONTINUE_TXN)
		a7NoACK = generate(A, 7, Flag_CONTINUE_TXN) // Not included in a6ACK.
		a6ACK   = generate(A, 6, Flag_ACK_TXN)      // Served from ring.
	)
	queue(seq, a5, a7NoACK, a6ACK)
	expectDeque(t, seq, a5, a6ACK)

	// Case: Rollback with interleaved producer B.
	var (
		b1         = generate(B, 1, Flag_CONTINUE_TXN) // Evicted.
		a7Rollback = generate(A, 7, Flag_CONTINUE_TXN) // Evicted.
		a8Rollback = generate(A, 8, Flag_CONTINUE_TXN)
		b2         = generate(B, 2, Flag_CONTINUE_TXN)
		a6Abort    = generate(A, 6, Flag_ACK_TXN) // Aborts back to SeqNo 6.
	)
	queue(seq, a7Rollback, b1, a7Rollback, a8Rollback, b2, a6Abort)
	expectDeque(t, seq) // No messages deque.

	// Case: Interleaved producer ACKs. A replay is required due to eviction.
	var b3ACK = generate(B, 3, Flag_ACK_TXN)
	queue(seq, b3ACK)
	expectReplay(t, seq, b1.Begin, b2.Begin, b1, a7Rollback, a8Rollback)
	expectDeque(t, seq, b1, b2, b3ACK)

	// Case: Sequence which requires replay, with duplicates internal
	// to the sequence and from before it, which are encountered in
	// the ring and also during replay.
	var (
		b4    = generate(B, 4, Flag_CONTINUE_TXN) // Evicted.
		b1Dup = generate(B, 1, Flag_CONTINUE_TXN)
		b4Dup = generate(B, 4, Flag_CONTINUE_TXN)
		b5    = generate(B, 5, Flag_CONTINUE_TXN) // Evicted.
		b6    = generate(B, 6, Flag_CONTINUE_TXN)
		b2Dup = generate(B, 2, Flag_CONTINUE_TXN)
		b7    = generate(B, 7, Flag_CONTINUE_TXN)
		b8ACK = generate(B, 8, Flag_ACK_TXN)
	)
	queue(seq, b4, b1Dup, b4Dup, b5, b6, b2Dup, b7, b8ACK)
	expectReplay(t, seq, b4.Begin, b6.Begin, b4, b1Dup, b4Dup, b5)
	expectDeque(t, seq, b4, b5, b6, b7, b8ACK)

	// Case: Partial rollback where all ring entries are skipped.
	var (
		b9       = generate(B, 9, Flag_CONTINUE_TXN)  // Evicted.
		b11NoACK = generate(B, 11, Flag_CONTINUE_TXN) // Evicted
		b12NoACK = generate(B, 12, Flag_CONTINUE_TXN)
		b13NoACK = generate(B, 13, Flag_CONTINUE_TXN)
		b10ACK   = generate(B, 10, Flag_ACK_TXN)
	)
	queue(seq, b9, b11NoACK, b12NoACK, b13NoACK, b10ACK)
	expectReplay(t, seq, b9.Begin, b12NoACK.Begin, b9, b11NoACK)
	expectDeque(t, seq, b9, b10ACK)

	// Case: Interleaved ACK'd sequences requiring two replays.
	var (
		b11    = generate(B, 11, Flag_CONTINUE_TXN) // Evicted.
		a7     = generate(A, 7, Flag_CONTINUE_TXN)  // Evicted.
		a8     = generate(A, 8, Flag_CONTINUE_TXN)
		b12    = generate(B, 12, Flag_CONTINUE_TXN)
		a9ACK  = generate(A, 9, Flag_ACK_TXN)
		b13ACK = generate(B, 13, Flag_ACK_TXN)
	)
	queue(seq, b11, a7, a8, b12, a9ACK)
	expectReplay(t, seq, a7.Begin, a8.Begin, a7)
	expectDeque(t, seq, a7, a8, a9ACK)

	queue(seq, b13ACK)
	expectReplay(t, seq, b11.Begin, b12.Begin, b11, a7, a8)
	expectDeque(t, seq, b11, b12, b13ACK)

	// Case: Reset to earlier ACK, followed by re-use of SeqNos.
	var (
		b8ACKReset  = generate(B, 8, Flag_ACK_TXN)
		b9Reuse     = generate(B, 9, Flag_CONTINUE_TXN)
		b10ACKReuse = generate(B, 10, Flag_ACK_TXN)
	)

	queue(seq, b8ACKReset, b9Reuse, b10ACKReuse)
	expectDeque(t, seq, b9Reuse, b10ACKReuse)
}

func TestSequencerTxnWithoutBuffer(t *testing.T) {
	var (
		generate = newTestMsgGenerator()
		seq      = NewSequencer(nil, 0)
		A, B     = NewProducerID(), NewProducerID()

		a1    = generate(A, 1, Flag_CONTINUE_TXN)
		a2    = generate(A, 2, Flag_CONTINUE_TXN)
		b1    = generate(B, 1, Flag_CONTINUE_TXN)
		a1Dup = generate(A, 1, Flag_CONTINUE_TXN)
		a2Dup = generate(A, 2, Flag_CONTINUE_TXN)
		a3ACK = generate(A, 3, Flag_ACK_TXN)
		b2    = generate(B, 2, Flag_CONTINUE_TXN)
		b3ACK = generate(B, 3, Flag_ACK_TXN)
	)
	queue(seq, a1, a2, b1, a1Dup, a2Dup, a3ACK)
	expectReplay(t, seq, a1.Begin, a3ACK.Begin, a1, a2, b1, a1Dup, a2Dup)
	expectDeque(t, seq, a1, a2, a3ACK)

	queue(seq, b2, b3ACK)
	expectReplay(t, seq, b1.Begin, b3ACK.Begin, b1, a1Dup, a2Dup, a3ACK, b2)
	expectDeque(t, seq, b1, b2, b3ACK)
}

func TestSequencerOutsideTxnCases(t *testing.T) {
	var (
		generate = newTestMsgGenerator()
		seq      = NewSequencer(nil, 0)
		A        = NewProducerID()
	)

	// Case: OUTSIDE_TXN messages immediately deque.
	var (
		a1 = generate(A, 1, Flag_OUTSIDE_TXN)
		a2 = generate(A, 2, Flag_OUTSIDE_TXN)
	)
	queue(seq, a1)
	expectDeque(t, seq, a1)
	queue(seq, a2)
	expectDeque(t, seq, a2)

	// Case: Duplicates are ignored.
	var (
		a1Dup = generate(A, 1, Flag_OUTSIDE_TXN)
		a2Dup = generate(A, 2, Flag_OUTSIDE_TXN)
	)
	queue(seq, a1Dup, a2Dup)
	expectDeque(t, seq)

	// Case: Any preceding CONTINUE_TXN messages are aborted.
	var (
		a3Discard = generate(A, 3, Flag_CONTINUE_TXN)
		a4Discard = generate(A, 4, Flag_CONTINUE_TXN)
		a5        = generate(A, 5, Flag_OUTSIDE_TXN)
	)
	queue(seq, a3Discard, a4Discard, a5)
	expectDeque(t, seq, a5)

	// Case: Messages with unknown flags are treated as OUTSIDE_TXN.
	var (
		a6Discard    = generate(A, 6, Flag_CONTINUE_TXN)
		a7BadBits    = generate(A, 7, 0x100)
		a7BadBitsDup = generate(A, 7, 0x100)
	)
	queue(seq, a6Discard, a7BadBits)
	expectDeque(t, seq, a7BadBits)
	queue(seq, a7BadBitsDup)
	expectDeque(t, seq)

	// Case: Messages with a zero UUID always deque.
	var (
		z1 = generate(ProducerID{}, 0, 0)
		z2 = generate(ProducerID{}, 0, 0)
	)
	z1.SetUUID(UUID{})
	z2.SetUUID(UUID{})

	queue(seq, z1)
	expectDeque(t, seq, z1)
	queue(seq, z2)
	expectDeque(t, seq, z2)
}

func TestSequencerProducerStatesRoundTrip(t *testing.T) {
	var (
		generate = newTestMsgGenerator()
		seq1     = NewSequencer(nil, 12)
		A, B, C  = NewProducerID(), NewProducerID(), NewProducerID()
		jpA      = JournalProducer{Journal: "test/journal", Producer: A}
		jpB      = JournalProducer{Journal: "test/journal", Producer: B}
		jpC      = JournalProducer{Journal: "test/journal", Producer: C}

		a1         = generate(A, 1, Flag_CONTINUE_TXN)
		a2         = generate(A, 2, Flag_CONTINUE_TXN)
		b1         = generate(B, 1, Flag_CONTINUE_TXN)
		b2         = generate(B, 2, Flag_CONTINUE_TXN)
		c1ACK      = generate(C, 1, Flag_ACK_TXN)
		b3ACK      = generate(B, 3, Flag_ACK_TXN)
		c2         = generate(C, 2, Flag_CONTINUE_TXN)
		c1Rollback = generate(C, 1, Flag_ACK_TXN)
		a3ACK      = generate(A, 3, Flag_ACK_TXN)
	)
	queue(seq1, a1, a2, b1, b2, c1ACK)
	expectDeque(t, seq1, c1ACK)

	var states = seq1.ProducerStates()
	var expect = []ProducerState{
		{JournalProducer: jpA, Begin: a1.Begin, LastAck: 0},
		{JournalProducer: jpB, Begin: b1.Begin, LastAck: 0},
		{JournalProducer: jpC, Begin: -1, LastAck: 1},
	}
	sort.Slice(states, func(i, j int) bool {
		return bytes.Compare(states[i].Producer[:], states[j].Producer[:]) < 0
	})
	sort.Slice(expect, func(i, j int) bool {
		return bytes.Compare(expect[i].Producer[:], expect[j].Producer[:]) < 0
	})
	assert.Equal(t, expect, states)

	// Recover Sequencer from persisted states.
	var seq2 = NewSequencer(states, 12)

	// Expect both Sequencers produce the same output from here,
	// though |seq2| requires replays while |seq1| does not.
	queue(seq1, b3ACK)
	queue(seq2, b3ACK)
	expectReplay(t, seq2, b1.Begin, b3ACK.Begin, b1, b2, c1ACK)

	expectDeque(t, seq1, b1, b2, b3ACK)
	expectDeque(t, seq2, b1, b2, b3ACK)

	queue(seq1, c2, c1Rollback)
	queue(seq2, c2, c1Rollback)

	expectDeque(t, seq1)
	expectDeque(t, seq2)

	queue(seq1, a3ACK)
	queue(seq2, a3ACK)
	expectReplay(t, seq2, a1.Begin, a3ACK.Begin, a1, a2, b1, b2, c1ACK)

	expectDeque(t, seq1, a1, a2, a3ACK)
	expectDeque(t, seq2, a1, a2, a3ACK)
}

func TestSequencerReplayReaderErrors(t *testing.T) {
	var A, B = NewProducerID(), NewProducerID()
	var cases = []struct {
		wrap   func(Iterator) func() (Envelope, error)
		expect string
	}{
		{ // No error.
			wrap:   func(it Iterator) func() (Envelope, error) { return it.Next },
			expect: "",
		},
		{ // Reader errors are passed through.
			wrap: func(Iterator) func() (Envelope, error) {
				return func() (Envelope, error) {
					return Envelope{}, io.ErrUnexpectedEOF
				}
			},
			expect: "replay reader: unexpected EOF",
		},
		{ // Returns Envelope of the wrong journal.
			wrap: func(it Iterator) func() (Envelope, error) {
				return func() (env Envelope, err error) {
					env, err = it.Next()
					env.Journal.Name = "wrong/journal"
					return
				}
			},
			expect: "replay reader: wrong journal (wrong/journal; expected test/journal)",
		},
		{ // Returns a Begin that's before the ReplayRange.
			wrap: func(it Iterator) func() (Envelope, error) {
				return func() (env Envelope, err error) {
					env, err = it.Next()
					env.Begin -= 1
					return
				}
			},
			expect: "replay reader: wrong Begin (101; expected >= 102)",
		},
		{ // Returns an End that's after the ReplayRange.
			wrap: func(it Iterator) func() (Envelope, error) {
				return func() (env Envelope, err error) {
					env, err = it.Next()
					env.End += 100
					return
				}
			},
			expect: "replay reader: wrong End (302; expected <= 204)",
		},
	}
	for _, tc := range cases {
		var (
			generate = newTestMsgGenerator()
			seq      = NewSequencer(nil, 0)
			b0       = generate(B, 0, Flag_CONTINUE_TXN)
			a1       = generate(A, 1, Flag_CONTINUE_TXN)
			a2ACK    = generate(A, 2, Flag_ACK_TXN)
		)
		queue(seq, b0, a1, a2ACK)
		expectReplay(t, seq, a1.Begin, a2ACK.Begin, a1)

		seq.replay = fnIterator(tc.wrap(seq.replay))
		var _, err = seq.DequeCommitted()

		if tc.expect == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, tc.expect)
		}
	}
}

type fnIterator func() (Envelope, error)

func (fn fnIterator) Next() (Envelope, error) { return fn() }

func queue(seq *Sequencer, envs ...Envelope) {
	for _, e := range envs {
		seq.QueueUncommitted(e)
	}
}

func expectDeque(t *testing.T, seq *Sequencer, expect ...Envelope) {
	for len(expect) != 0 {
		var env, err = seq.DequeCommitted()

		assert.NoError(t, err)
		assert.Equal(t, expect[0], env)
		expect = expect[1:]
	}
	var _, err = seq.DequeCommitted()
	assert.Equal(t, io.EOF, err)
}

func expectReplay(t *testing.T, seq *Sequencer, expectBegin, expectEnd pb.Offset, envs ...Envelope) {
	var _, err = seq.DequeCommitted()
	assert.Equal(t, ErrMustStartReplay, err)

	var begin, end = seq.ReplayRange()
	assert.Equal(t, expectBegin, begin)
	assert.Equal(t, expectEnd, end)

	seq.StartReplay(fnIterator(func() (env Envelope, err error) {
		if envs == nil {
			panic("unexpected extra replay call")
		} else if len(envs) == 0 {
			envs, err = nil, io.EOF
			return
		} else {
			env, envs = envs[0], envs[1:]
			return
		}
	}))
}

func newTestMsgGenerator() func(p ProducerID, clock Clock, flags Flags) Envelope {
	var offset pb.Offset

	return func(p ProducerID, clock Clock, flags Flags) (e Envelope) {
		e = Envelope{
			Journal: &pb.JournalSpec{Name: "test/journal"},
			Begin:   offset,
			End:     offset + 100,
			Message: &testMsg{
				UUID: BuildUUID(p, clock, flags),
				Str:  strconv.Itoa(int(clock)),
			},
		}
		// Leave 2 bytes of dead space. Sequencer must handle non-contiguous envelopes.
		offset += 102
		return
	}
}

// testMsg meets the Message, Validator, & NewMessageFunc interfaces.
type testMsg struct {
	UUID UUID
	Str  string `json:",omitempty"`
	err  error
}

func newTestMsg(*pb.JournalSpec) (Message, error)        { return new(testMsg), nil }
func (m *testMsg) GetUUID() UUID                         { return m.UUID }
func (m *testMsg) SetUUID(uuid UUID)                     { m.UUID = uuid }
func (m *testMsg) NewAcknowledgement(pb.Journal) Message { return new(testMsg) }
func (m *testMsg) Validate() error                       { return m.err }
