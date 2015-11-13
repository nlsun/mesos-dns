package records

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records/labels"
	"github.com/mesosphere/mesos-dns/records/state"
	"crypto/sha1"
	"encoding/base32"
"strings"
	"hash/fnv"
	"strconv"
)

func init() {
	logging.VerboseFlag = false
	logging.VeryVerboseFlag = false
	logging.SetupLogs()
}

func TestMasterRecord(t *testing.T) {
	// masterRecord(domain string, masters []string, leader string)
	type expectedRR struct {
		name  string
		host  string
		rtype string
	}
	tt := []struct {
		domain  string
		masters []string
		leader  string
		expect  []expectedRR
	}{
		{"foo.com", nil, "", nil},
		{"foo.com", nil, "@", nil},
		{"foo.com", nil, "1@", nil},
		{"foo.com", nil, "@2", nil},
		{"foo.com", nil, "3@4", nil},
		{"foo.com", nil, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master0.foo.com.", "6", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// single master: leader and fallback
		{"foo.com", []string{"6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master0.foo.com.", "6", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader not in fallback list
		{"foo.com", []string{"8:9"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// duplicate fallback masters, leader not in fallback list
		{"foo.com", []string{"8:9", "8:9"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader that's also listed in the fallback list (at the end)
		{"foo.com", []string{"8:9", "6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// duplicate leading masters in the fallback list
		{"foo.com", []string{"8:9", "6:7", "6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader that's also listed in the fallback list (in the middle)
		{"foo.com", []string{"8:9", "6:7", "bob:0"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master.foo.com.", "bob", "A"},
				{"master0.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master2.foo.com.", "bob", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
	}
	for i, tc := range tt {
		rg := &RecordGenerator{}
		rg.As = make(rrs)
		rg.SRVs = make(rrs)
		t.Logf("test case %d", i+1)
		rg.masterRecord(tc.domain, tc.masters, tc.leader)
		if tc.expect == nil {
			if len(rg.As) > 0 {
				t.Fatalf("test case %d: unexpected As: %v", i+1, rg.As)
			}
			if len(rg.SRVs) > 0 {
				t.Fatalf("test case %d: unexpected SRVs: %v", i+1, rg.SRVs)
			}
		}
		expectedA := make(rrs)
		expectedSRV := make(rrs)
		for _, e := range tc.expect {
			found := rg.exists(e.name, e.host, e.rtype)
			if !found {
				t.Fatalf("test case %d: missing expected record: name=%q host=%q rtype=%s, As=%v", i+1, e.name, e.host, e.rtype, rg.As)
			}
			if e.rtype == "A" {
				expectedA[e.name] = append(expectedA[e.name], e.host)
			} else {
				expectedSRV[e.name] = append(expectedSRV[e.name], e.host)
			}
		}
		if !reflect.DeepEqual(rg.As, expectedA) {
			t.Fatalf("test case %d: expected As of %v instead of %v", i+1, expectedA, rg.As)
		}
		if !reflect.DeepEqual(rg.SRVs, expectedSRV) {
			t.Fatalf("test case %d: expected SRVs of %v instead of %v", i+1, expectedSRV, rg.SRVs)
		}
	}
}

func TestLeaderIP(t *testing.T) {
	l := "master@144.76.157.37:5050"

	ip := leaderIP(l)

	if ip != "144.76.157.37" {
		t.Error("not parsing ip")
	}
}

func testRecordGenerator(t *testing.T, spec labels.Func, ipSources []string) RecordGenerator {
	var sj state.State

	b, err := ioutil.ReadFile("../factories/fake.json")
	if err != nil {
		t.Fatal(err)
	} else if err = json.Unmarshal(b, &sj); err != nil {
		t.Fatal(err)
	}

	sj.Leader = "master@144.76.157.37:5050"
	masters := []string{"144.76.157.37:5050"}

	var rg RecordGenerator
	if err := rg.InsertState(sj, "mesos", "mesos-dns.mesos.", "127.0.0.1", masters, ipSources, spec); err != nil {
		t.Fatal(err)
	}

	return rg
}

// ensure we are parsing what we think we are
func TestInsertState(t *testing.T) {
	rg := testRecordGenerator(t, labels.RFC952, []string{"docker", "mesos", "host"})
	rgDocker := testRecordGenerator(t, labels.RFC952, []string{"docker", "host"})
	rgMesos := testRecordGenerator(t, labels.RFC952, []string{"mesos", "host"})
	rgSlave := testRecordGenerator(t, labels.RFC952, []string{"host"})

	for i, tt := range []struct {
		rrs  rrs
		name string
		want []string
	}{
		{rg.As, "liquor-store.marathon.mesos.", []string{"10.3.0.1", "10.3.0.2"}},
		{rg.As, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rg.As, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},
		{rg.As, "nginx.marathon.mesos.", []string{"10.3.0.3"}},
		{rg.As, "poseidon.marathon.mesos.", nil},
		{rg.As, "poseidon.marathon.slave.mesos.", nil},
		{rg.As, "master.mesos.", []string{"144.76.157.37"}},
		{rg.As, "master0.mesos.", []string{"144.76.157.37"}},
		{rg.As, "leader.mesos.", []string{"144.76.157.37"}},
		{rg.As, "slave.mesos.", []string{"1.2.3.10", "1.2.3.11", "1.2.3.12"}},
		{rg.As, "some-box.chronoswithaspaceandmixe.mesos.", []string{"1.2.3.11"}}, // ensure we translate the framework name as well
		{rg.As, "marathon.mesos.", []string{"1.2.3.11"}},
		{rg.SRVs, "_poseidon._tcp.marathon.mesos.", nil},
		{rg.SRVs, "_leader._tcp.mesos.", []string{"leader.mesos.:5050"}},
		{rg.SRVs, "_liquor-store._tcp.marathon.mesos.", []string{
			"liquor-store-d0ca-0.marathon.mesos.:80",
			"liquor-store-d0ca-0.marathon.mesos.:443",
			"liquor-store-be2c-1.marathon.mesos.:80",
			"liquor-store-be2c-1.marathon.mesos.:443",
		}},
		{rg.SRVs, "_liquor-store._udp.marathon.mesos.", nil},
		{rg.SRVs, "_liquor-store.marathon.mesos.", nil},
		{rg.SRVs, "_car-store._tcp.marathon.mesos.", []string{
			"car-store-bd45-0.marathon.slave.mesos.:31364",
			"car-store-bd45-0.marathon.slave.mesos.:31365",
		}},
		{rg.SRVs, "_car-store._udp.marathon.mesos.", []string{
			"car-store-bd45-0.marathon.slave.mesos.:31364",
			"car-store-bd45-0.marathon.slave.mesos.:31365",
		}},
		{rg.SRVs, "_slave._tcp.mesos.", []string{"slave.mesos.:5051"}},
		{rg.SRVs, "_framework._tcp.marathon.mesos.", []string{"marathon.mesos.:25501"}},

		{rgSlave.As, "liquor-store.marathon.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgSlave.As, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgSlave.As, "nginx.marathon.mesos.", []string{"1.2.3.11"}},
		{rgSlave.As, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},

		{rgMesos.As, "liquor-store.marathon.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgMesos.As, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgMesos.As, "nginx.marathon.mesos.", []string{"10.3.0.3"}},
		{rgMesos.As, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},

		{rgDocker.As, "liquor-store.marathon.mesos.", []string{"10.3.0.1", "10.3.0.2"}},
		{rgDocker.As, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgDocker.As, "nginx.marathon.mesos.", []string{"1.2.3.11"}},
		{rgDocker.As, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},
	} {
		if got := tt.rrs[tt.name]; !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d: %q: got: %q, want: %q", i, tt.name, got, tt.want)
		}
	}
}

// ensure we only generate one A record for each host
func TestNTasks(t *testing.T) {
	rg := &RecordGenerator{}
	rg.As = make(rrs)

	rg.insertRR("blah.mesos", "10.0.0.1", "A")
	rg.insertRR("blah.mesos", "10.0.0.1", "A")
	rg.insertRR("blah.mesos", "10.0.0.2", "A")

	k, _ := rg.As["blah.mesos"]

	if len(k) != 2 {
		t.Error("should only have 2 A records")
	}
}

func TestHashStringSha(t *testing.T) {

	fn := func(a, b string) bool {
		return (a == b) || (hashStringSha(a) != hashStringSha(b))
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 1e6}); err != nil {
		t.Fatal(err)
	}
}


// Outputs a lowercase truncated sha1 sum in the extended hex alphabet
func hashStringSha(s string) string {
	hash := sha1.Sum([]byte(s))
	out := base32.HexEncoding.EncodeToString(hash[:])
	return strings.ToLower(out[:6])
}

func BenchmarkHashSHA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hashStringSha(big)
	}
}

var big = "means that the loop ran 10000000 times at a speed of 282 ns per loop."

func BenchmarkHashFNV64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hashStringFNV64(big)
	}
}

func TestHashStringFNV64(t *testing.T) {

	fn := func(a, b string) bool {
		return (a == b) || (hashStringFNV64(a) != hashStringFNV64(b))
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 1e6}); err != nil {
		t.Fatal(err)
	}
}


// Outputs a lowercase truncated sha1 sum in the extended hex alphabet
func hashStringFNV64(s string) string {
	hasher64 := fnv.New64a()
	hasher64.Write([]byte(s))
	hash := hasher64.Sum64()
	foldedHash := int64(hash>>32 ^ (hash & 0xffffffff))
	ack := strconv.FormatInt(foldedHash, 32)
	return strings.ToLower(ack)
}

func BenchmarkHashFNV128(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hashStringFNV128(big)
	}
}

func TestHashStringFNV128(t *testing.T) {

	fn := func(a, b string) bool {
		return (a == b) || (hashStringFNV128(a) != hashStringFNV128(b))
	}
	if err := quick.Check(fn, &quick.Config{MaxCount: 1e6}); err != nil {
		t.Fatal(err)
	}
}


// Outputs a lowercase truncated sha1 sum in the extended hex alphabet
func hashStringFNV128(s string) string {
	hasher128 := fnv.New128()
	hasher64.Write([]byte(s))
	hash := hasher64.Sum128()
	foldedHash := int64(hash>>32 ^ (hash & 0xffffffff))
	ack := strconv.FormatInt(foldedHash, 32)
	return strings.ToLower(ack)
}

