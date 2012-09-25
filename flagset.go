package goptions

import (
	"fmt"
	"reflect"
	"strings"
)

type flagSet struct {
	helpFlag *flag
	shortMap map[string]*flag
	longMap  map[string]*flag
	flags    []*flag
	verbs    map[string]*flagSet
}

func newFlagSet(structValue reflect.Value) (*flagSet, error) {
	r := &flagSet{
		flags: make([]*flag, 0),
		verbs: make(map[string]*flagSet),
	}

	var i int
	// Parse Option fields
	for i = 0; i < structValue.Type().NumField(); i++ {
		tag := structValue.Type().Field(i).Tag.Get("goptions")
		fieldValue := structValue.Field(i)
		if fieldValue.Type().Name() == "Verbs" {
			break
		}
		flag, err := parseTag(tag)
		if err != nil {
			return nil, fmt.Errorf("Invalid tagline: %s", err)
		}
		flag.Value = fieldValue

		switch flag.Value.Interface().(type) {
		case Help:
			r.helpFlag = flag
		}

		r.flags = append(r.flags, flag)
	}
	r.shortMap, r.longMap = r.shortFlagMap(), r.longFlagMap()

	// Parse verb fields
	for i++; i < structValue.Type().NumField(); i++ {
		fieldValue := structValue.Field(i)
		tag := structValue.Type().Field(i).Tag.Get("goptions")
		fs, err := newFlagSet(fieldValue)
		if err != nil {
			return nil, fmt.Errorf("Invalid verb: %s", err)
		}
		r.verbs[tag] = fs
	}

	return r, nil
}

func (fs *flagSet) shortFlagMap() map[string]*flag {
	r := make(map[string]*flag)
	for _, flag := range fs.flags {
		for _, short := range flag.Short {
			r[short] = flag
		}
	}
	return r
}

func (fs *flagSet) longFlagMap() map[string]*flag {
	r := make(map[string]*flag)
	for _, flag := range fs.flags {
		for _, long := range flag.Long {
			r[long] = flag
		}
	}
	return r
}

func (fs *flagSet) MutexGroups() map[string][]*flag {
	r := make(map[string][]*flag)
	for _, f := range fs.flags {
		mg := f.MutexGroup
		if len(mg) == 0 {
			continue
		}
		if _, ok := r[mg]; !ok {
			r[mg] = make([]*flag, 0)
		}
		r[mg] = append(r[mg], f)
	}
	return r
}

func (fs *flagSet) Parse(args []string) error {
	for len(args) > 0 {
		flags, restArgs, err := fs.parseNextItem(args)
		if err != nil {
			return err
		}
		fs.flags = append(fs.flags, flags...)
		args = restArgs
	}
	return nil
}

func (fs *flagSet) parseNextItem(args []string) ([]*flag, []string, error) {
	if strings.HasPrefix(args[0], "--") {
		return fs.parseLongFlag(args)
	} else if strings.HasPrefix(args[0], "-") {
		return fs.parseShortFlagCluster(args)
	} else {
		verb, ok := fs.verbs[args[0]]
		if !ok {
			return nil, args, fmt.Errorf("Unknown verb: %s", args[0])
		}
		err := verb.Parse(args[1:])
		if err != nil {
			return nil, args, err
		}
		return []*flag{}, []string{}, nil
	}
	panic("Invalid execution path")
}

func (fs *flagSet) parseLongFlag(args []string) ([]*flag, []string, error) {
	longflagname := args[0][2:]
	f, ok := fs.longMap[longflagname]
	if !ok {
		return nil, args, fmt.Errorf("Unknown flag --%s", longflagname)
	}
	args = args[1:]
	f.Set()
	if f.NeedsExtraValue() {
		err := f.SetValue(args[0])
		if err != nil {
			return nil, args, err
		}
		args = args[1:]
	}
	return []*flag{f}, args, nil
}

func (fs *flagSet) parseShortFlagCluster(args []string) ([]*flag, []string, error) {
	shortflagnames := args[0][1:]
	args = args[1:]
	r := make([]*flag, 0, len(shortflagnames))
	for idx, shortflagname := range shortflagnames {
		flag, ok := fs.shortMap[string(shortflagname)]
		if !ok {
			return nil, args, fmt.Errorf("Unknown flag -%s", string(shortflagname))
		}
		flag.Set()
		// If value-flag is given but is not the last in a short flag cluster,
		// it's an error.
		if flag.NeedsExtraValue() && idx != len(shortflagnames)-1 {
			return nil, args, fmt.Errorf("Flag %s needs a value", flag.Name())
		} else if flag.NeedsExtraValue() {
			err := flag.SetValue(args[0])
			if err != nil {
				return nil, args, err
			}
			args = args[1:]
		}
		r = append(r, flag)
	}
	return r, args, nil
}