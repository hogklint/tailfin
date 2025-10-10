//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package tailfincmd

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cast"
)

func toTimeMilli(a any) time.Time {
	t, _ := toTimeMilliE(a)
	return t
}

func toTimeMilliE(a any) (time.Time, error) {
	switch v := a.(type) {
	case string:
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.UnixMilli(ms), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return time.Time{}, err
		}
		return time.UnixMilli(i), nil
	case float64:
		return time.UnixMilli(int64(v)), nil
	}
	return time.Time{}, errors.New("unsupported type")
}

func toTime(a any) time.Time {
	t, _ := toTimeE(a)
	return t
}

func toTimeE(a any) (time.Time, error) {
	switch v := a.(type) {
	case string:
		if t, ok := parseUnixTimeNanoString(v); ok {
			return t, nil
		}
	case json.Number:
		if t, ok := parseUnixTimeNanoString(v.String()); ok {
			return t, nil
		}
	}
	return cast.ToTimeE(a)
}

func parseUnixTimeNanoString(num string) (time.Time, bool) {
	parts := strings.Split(num, ".")
	if len(parts) > 2 {
		return time.Time{}, false
	}

	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, false
	}

	var nsec int64
	if len(parts) == 2 {
		// convert fraction part to nanoseconds
		const digits = 9
		frac := parts[1]
		if len(frac) > digits {
			frac = frac[:digits]
		} else if len(frac) < digits {
			frac = frac + strings.Repeat("0", digits-len(frac))
		}
		nsec, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
	}
	return time.Unix(sec, nsec), true
}
