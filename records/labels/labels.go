package labels

import (
	"bytes"
	"strings"
)

// Sep is the default domain fragment separator.
const Sep = "."

// DomainFrag mangles the given name in order to produce a valid domain fragment.
// A valid domain fragment will consist of one or more host name labels
// concatenated by the given separator.
func DomainFrag(name, sep string, label Func) string {
	var labels []string
	for _, part := range strings.Split(name, sep) {
		if lab := label(part); lab != "" {
			labels = append(labels, lab)
		}
	}
	return strings.Join(labels, sep)
}

// Func is a function type representing label functions.
type Func func(string) string

// RFC952 mangles a name to conform to the DNS label rules specified in RFC952.
// See http://www.rfc-base.org/txt/rfc-952.txt
func RFC952(name string) string {
	return string(label([]byte(name), 24, "-0123456789", "-"))
}

// RFC1123 mangles a name to conform to the DNS label rules specified in RFC1123.
// See http://www.rfc-base.org/txt/rfc-1123.txt
func RFC1123(name string) string {
	return string(stringTrimmerFSM([]byte(name), 63, "-", "-"))
	//return string(label([]byte(name), 63, "-", "-"))
}


//%% When stripping from the accumulator left and right are reversed because it's backwards
//label(_, [], Acc) ->
//    string:strip(Acc, left, $-);
//label(State, [Char0 | RestFragmentStr], Acc) when (Char0 >= $A andalso Char0 =< $Z) ->
//    Char1 = Char0 - ($A - $a),
//    label(State, [Char1 | RestFragmentStr], Acc);
//label(start, FragmentStr = [Char | _RestFragmentStr], Acc) when ?ALLOWED_CHAR_GUARD(Char) ->
//    label(middle, FragmentStr, Acc);
//label(middle, FragmentStr, Acc0) when length(Acc0) > 62 ->
//    Acc1 = string:strip(Acc0, left, $-),
//    label(terminate, FragmentStr, Acc1);
//label(middle, [Char | RestFragmentStr], Acc) when Char == $- orelse Char == $_ orelse Char == $. ->
//    label(middle, RestFragmentStr, [$- | Acc]);
//label(terminate, _Str, Acc) when length(Acc) == 63 ->
//    label(terminate, [], Acc);
//label(State, [Char | RestFragmentStr], Acc) when ?ALLOWED_CHAR_GUARD(Char) ->
//    label(State, RestFragmentStr, [Char | Acc]);
//label(State, [_Char | RestFragmentStr], Acc) ->
//    label(State, RestFragmentStr, Acc).
const (
	START = iota
	MIDDLE = iota
	END = iota
)

var ALLOWED_CHARS = []byte("0123456789abcdefghijklmnopqrstuvwxyz")
func stringTrimmerFSM(name []byte, maxlen int, left, right string) []byte {
	// Start state
	state := START
	accum := make([]byte, 0, len(name))
	name = bytes.ToLower(name)

	for {
		if len(name) == 0 {
			return bytes.TrimRight(accum, right)
		}

		switch(state)  {
		case START:
			if bytes.IndexByte(ALLOWED_CHARS, name[0]) > -1 && bytes.IndexAny(name[:1], left) == -1 {
				state = MIDDLE
				continue
			}
		case MIDDLE:
			if len(accum) >= maxlen {
				accum = bytes.TrimRight(accum, "-")
				state = END
				continue
			}
			if name[0] == '-' || name[0] == '_' || name[0] == '.' {
				accum = append(accum, '-')
				name = name[1:]
				continue
			}
		case END:
			if len(accum) == maxlen {
				name = []byte{}
				continue
			}
		}
		if bytes.IndexByte(ALLOWED_CHARS, name[0]) > -1 {
			accum = append(accum, name[0])
		}
		name = name[1:]
	}
}
// label computes a label from the given name with maxlen length and the
// left and right cutsets trimmed from their respective ends.
func label(name []byte, maxlen int, left, right string) []byte {
	return trimCut(bytes.Map(mapping, name), maxlen, left, right)
}

// mapping maps a given rune to its valid DNS label counterpart.
func mapping(r rune) rune {
	switch {
	case r >= 'A' && r <= 'Z':
		return r - ('A' - 'a')
	case r >= 'a' && r <= 'z':
		fallthrough
	case r >= '0' && r <= '9':
		return r
	case r == '-' || r == '.' || r == '_':
		return '-'
	default:
		return -1
	}
}

// trimCut cuts the given label at min(maxlen, len(label)) and ensures the left
// and right cutsets are trimmed from their respective ends.
func trimCut(label []byte, maxlen int, left, right string) []byte {
	trim := bytes.TrimLeft(label, left)
	size := min(len(trim), maxlen)
	head := bytes.TrimRight(trim[:size], right)
	if len(head) == size {
		return head
	}
	tail := bytes.TrimLeft(trim[size:], right)
	if len(tail) > 0 {
		return append(head, tail[:size-len(head)]...)
	}
	return head
}

// min returns the minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
