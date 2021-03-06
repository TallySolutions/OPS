package lepton

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

var localManifestDir = path.Join(GetOpsHome(), "manifests")

// link refers to a link filetype
type link struct {
	path string
}

// ManifestNetworkConfig has network configuration to set static IP
type ManifestNetworkConfig struct {
	IP      string
	Gateway string
	NetMask string
}

// Manifest represent the filesystem.
type Manifest struct {
	sb            strings.Builder
	children      map[string]interface{} // root fs
	boot          map[string]interface{} // boot fs
	program       string
	args          []string
	debugFlags    map[string]rune
	noTrace       []string
	environment   map[string]string
	targetRoot    string
	mounts        map[string]string
	klibs         []string
	nightly       bool
	networkConfig *ManifestNetworkConfig
}

// NewManifest init
func NewManifest(targetRoot string) *Manifest {
	return &Manifest{
		boot:        make(map[string]interface{}),
		children:    make(map[string]interface{}),
		debugFlags:  make(map[string]rune),
		environment: make(map[string]string),
		targetRoot:  targetRoot,
		mounts:      make(map[string]string),
	}
}

// AddNetworkConfig adds network configuration
func (m *Manifest) AddNetworkConfig(networkConfig *ManifestNetworkConfig) {
	m.networkConfig = networkConfig
}

// AddUserProgram adds user program
func (m *Manifest) AddUserProgram(imgpath string) {
	parts := strings.Split(imgpath, "/")
	if parts[0] == "." {
		parts = parts[1:]
	}
	m.program = path.Join("/", path.Join(parts...))
	err := m.AddFile(m.program, imgpath)
	if err != nil {
		panic(err)
	}
}

// AddMount adds mount
func (m *Manifest) AddMount(label, path string) {
	dir := strings.TrimPrefix(path, "/")
	m.children[dir] = map[string]interface{}{}
	m.mounts[label] = path
}

// AddEnvironmentVariable adds environment variables
func (m *Manifest) AddEnvironmentVariable(name string, value string) {
	m.environment[name] = value

	if name == "RADAR_KEY" {
		m.AddKlibs([]string{"tls", "radar"})
	}

}

// AddKlibs append klibs to manifest file if they don't exist
func (m *Manifest) AddKlibs(klibs []string) {
	for _, klib := range klibs {
		var exists bool
		for _, mKlib := range m.klibs {
			if mKlib == klib {
				exists = true
				break
			}
		}

		if !exists {
			m.klibs = append(m.klibs, klib)
		}
	}
}

// AddArgument add commandline arguments to
// user program
func (m *Manifest) AddArgument(arg string) {
	m.args = append(m.args, arg)
}

// AddDebugFlag enables debug flags
func (m *Manifest) AddDebugFlag(name string, value rune) {
	m.debugFlags[name] = value
}

// AddNoTrace enables debug flags
func (m *Manifest) AddNoTrace(name string) {
	m.noTrace = append(m.noTrace, name)
}

// AddKernel the kernel to use
func (m *Manifest) AddKernel(path string) {
	node := make(map[string]interface{})
	node["kernel"] = path
	m.boot = node
}

// AddRelative path
func (m *Manifest) AddRelative(key string, path string) {
	m.children[key] = path
}

// AddDirectory adds all files in dir to image
func (m *Manifest) AddDirectory(dir string) error {
	err := filepath.Walk(dir, func(hostpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// if the path is relative then root it to image path
		var vmpath string
		if hostpath[0] != '/' {
			vmpath = "/" + hostpath
		} else {
			vmpath = hostpath
		}

		if (info.Mode() & os.ModeSymlink) != 0 {
			info, err = os.Stat(hostpath)
			if err != nil {
				fmt.Printf("warning: %v\n", err)
				// ignore invalid symlinks
				return nil
			}

			// add link and continue on
			err = m.AddLink(vmpath, hostpath)
			if err != nil {
				return err
			}

			return nil
		}

		if info.IsDir() {
			parts := strings.FieldsFunc(vmpath, func(c rune) bool { return c == '/' })
			node := m.children
			for i := 0; i < len(parts); i++ {
				if _, ok := node[parts[i]]; !ok {
					node[parts[i]] = make(map[string]interface{})
				}
				if reflect.TypeOf(node[parts[i]]).Kind() == reflect.String {
					err := fmt.Errorf("directory %s is conflicting with an existing file", hostpath)
					fmt.Println(err)
					return err
				}
				node = node[parts[i]].(map[string]interface{})
			}
		} else {
			err = m.AddFile(vmpath, hostpath)
			if err != nil {
				return err
			}
		}
		return nil

	})
	return err
}

// AddRelativeDirectory adds all files in dir to image
func (m *Manifest) AddRelativeDirectory(src string) error {
	err := filepath.Walk(src, func(hostpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		vmpath := "/" + strings.TrimPrefix(hostpath, src)

		if (info.Mode() & os.ModeSymlink) != 0 {
			info, err = os.Stat(hostpath)
			if err != nil {
				fmt.Printf("warning: %v\n", err)
				// ignore invalid symlinks
				return nil
			}

			// add link and continue on
			err = m.AddLink(vmpath, hostpath)
			if err != nil {
				return err
			}

			return nil
		}

		if info.IsDir() {
			parts := strings.FieldsFunc(vmpath, func(c rune) bool { return c == '/' })
			node := m.children
			for i := 0; i < len(parts); i++ {
				if _, ok := node[parts[i]]; !ok {
					node[parts[i]] = make(map[string]interface{})
				}
				if reflect.TypeOf(node[parts[i]]).Kind() == reflect.String {
					err := fmt.Errorf("directory %s is conflicting with an existing file", hostpath)
					fmt.Println(err)
					return err
				}
				node = node[parts[i]].(map[string]interface{})
			}
		} else {
			err = m.AddFile(vmpath, hostpath)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// FileExists checks if file is present at path in manifest
func (m *Manifest) FileExists(filepath string) bool {
	parts := strings.FieldsFunc(filepath, func(c rune) bool { return c == '/' })
	node := m.children
	for i := 0; i < len(parts)-1; i++ {
		if _, ok := node[parts[i]]; !ok {
			return false
		}
		node = node[parts[i]].(map[string]interface{})
	}
	pathtest := node[parts[len(parts)-1]]
	if pathtest != nil && reflect.TypeOf(pathtest).Kind() == reflect.String {
		return true
	}
	return false
}

// AddLink to add a file to manifest
func (m *Manifest) AddLink(filepath string, hostpath string) error {
	parts := strings.FieldsFunc(filepath, func(c rune) bool { return c == '/' })
	node := m.children

	for i := 0; i < len(parts)-1; i++ {
		if _, ok := node[parts[i]]; !ok {
			node[parts[i]] = make(map[string]interface{})
		}
		node = node[parts[i]].(map[string]interface{})
	}

	pathtest := node[parts[len(parts)-1]]
	if pathtest != nil && reflect.TypeOf(pathtest).Kind() != reflect.String {
		err := fmt.Errorf("file %s overriding an existing directory", filepath)
		fmt.Println(err)
		return err
	}

	if pathtest != nil && reflect.TypeOf(pathtest).Kind() == reflect.String && node[parts[len(parts)-1]] != hostpath {
		fmt.Printf("warning: overwriting existing file %s hostpath old: %s new: %s\n", filepath, node[parts[len(parts)-1]], hostpath)
	}

	_, err := lookupFile(m.targetRoot, hostpath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "please check your manifest for the missing file: %v\n", err)
			os.Exit(1)
		}
		return err
	}

	s, err := os.Readlink(hostpath)
	if err != nil {
		fmt.Println("bad link")
		os.Exit(1)
	}

	node[parts[len(parts)-1]] = link{path: s}
	return nil
}

// AddFile to add a file to manifest
func (m *Manifest) AddFile(filepath string, hostpath string) error {
	parts := strings.FieldsFunc(filepath, func(c rune) bool { return c == '/' })
	node := m.children

	for i := 0; i < len(parts)-1; i++ {
		if _, ok := node[parts[i]]; !ok {
			node[parts[i]] = make(map[string]interface{})
		}
		node = node[parts[i]].(map[string]interface{})
	}

	pathtest := node[parts[len(parts)-1]]
	if pathtest != nil && reflect.TypeOf(pathtest).Kind() != reflect.String {
		err := fmt.Errorf("file '%s' overriding an existing directory", filepath)
		fmt.Println(err)
		os.Exit(1)
	}

	if pathtest != nil && reflect.TypeOf(pathtest).Kind() == reflect.String && pathtest != hostpath {
		fmt.Printf("warning: overwriting existing file %s hostpath old: %s new: %s\n", filepath, pathtest, hostpath)
	}

	_, err := lookupFile(m.targetRoot, hostpath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "please check your manifest for the missing file: %v\n", err)
			os.Exit(1)
		}
		return err
	}

	node[parts[len(parts)-1]] = hostpath
	return nil
}

// AddLibrary to add a dependent library
func (m *Manifest) AddLibrary(path string) {
	parts := strings.FieldsFunc(path, func(c rune) bool { return c == '/' })
	node := m.children
	for i := 0; i < len(parts)-1; i++ {
		if _, ok := node[parts[i]]; !ok {
			node[parts[i]] = make(map[string]interface{})
		}
		node = node[parts[i]].(map[string]interface{})
	}
	node[parts[len(parts)-1]] = path
}

// AddUserData adds all files in dir to
// final image.
func (m *Manifest) AddUserData(dir string) {
	// TODO
}

func escapeValue(s string) string {
	if strings.Contains(s, "\"") {
		s = strings.Replace(s, "\"", "\\\"", -1)
	}
	if strings.ContainsAny(s, "\":()[] \t\n") {
		s = "\"" + s + "\""
	}
	return s
}

func (m *Manifest) String() string {
	sb := m.sb
	sb.WriteString("(\n")

	// write boot fs

	if len(m.boot) > 0 {
		sb.WriteString("boot:(children:(\n")
		toString(&m.boot, &sb, 4)

		// include klibs specified in configuration if present in ops klib directory
		if len(m.klibs) > 0 {
			klibs := map[string]interface{}{}
			klibsPath := getKlibsDir(m.nightly)
			if _, err := os.Stat(klibsPath); !os.IsNotExist(err) {

				sb.WriteString("    klib:(children:(\n")

				for _, klibName := range m.klibs {
					klibPath := klibsPath + "/" + klibName

					if _, err := os.Stat(klibPath); !os.IsNotExist(err) {
						klibs[klibName] = klibPath
					} else {
						fmt.Printf("Klib %s not found in directory %s\n", klibName, klibsPath)
					}
				}
				toString(&klibs, &sb, 6)

				sb.WriteString("    ))\n")
			} else {
				fmt.Printf("Klibs directory with path %s not found\n", klibsPath)
			}
		}

		sb.WriteString("))\n")
	}

	// write root fs
	sb.WriteString("children:(\n")
	toString(&m.children, &sb, 4)
	sb.WriteString(")\n")

	// program
	if m.program != "" {
		sb.WriteString("program:")
		sb.WriteString(m.program)
		sb.WriteRune('\n')
	}

	//
	if len(m.klibs) > 0 {
		sb.WriteString("klibs:bootfs\n")

		for _, klib := range m.klibs {
			if klib == "ntp" {
				var err error

				var ntpAddress string
				var ntpPort string
				var ntpPollMin string
				var ntpPollMax string

				var pollMinNumber int
				var pollMaxNumber int

				if val, ok := m.environment["ntpAddress"]; ok {
					ntpAddress = val
				}

				if val, ok := m.environment["ntpPort"]; ok {
					ntpPort = val
				}

				if val, ok := m.environment["ntpPollMin"]; ok {
					pollMinNumber, err = strconv.Atoi(val)
					if err == nil && pollMinNumber > 3 {
						ntpPollMin = val
					}
				}

				if val, ok := m.environment["ntpPollMax"]; ok {
					pollMaxNumber, err = strconv.Atoi(val)
					if err == nil && pollMaxNumber < 18 {
						ntpPollMax = val
					}
				}

				if pollMinNumber != 0 && pollMaxNumber != 0 && pollMinNumber > pollMaxNumber {
					ntpPollMin = ""
					ntpPollMax = ""
				}

				if ntpAddress != "" {
					sb.WriteString(fmt.Sprintf("ntp_address:%s\n", ntpAddress))
				}

				if ntpPort != "" {
					sb.WriteString(fmt.Sprintf("ntp_port:%s\n", ntpPort))
				}

				if ntpPollMin != "" {
					sb.WriteString(fmt.Sprintf("ntp_poll_min:%s\n", ntpPollMin))
				}

				if ntpPollMax != "" {
					sb.WriteString(fmt.Sprintf("ntp_poll_max:%s\n", ntpPollMax))
				}

				break
			}
		}

	}

	// arguments
	sb.WriteString("arguments:[")
	if len(m.args) > 0 {
		fmt.Println(m.args)
		escapedArgs := make([]string, len(m.args))
		for i, arg := range m.args {
			escapedArgs[i] = escapeValue(arg)
		}
		sb.WriteString(strings.Join(escapedArgs, " "))
	}
	sb.WriteString("]\n")

	// debug
	for k, v := range m.debugFlags {
		sb.WriteString(k)
		sb.WriteRune(':')
		sb.WriteRune(v)
		sb.WriteRune('\n')
	}

	// notrace
	if len(m.noTrace) > 0 {
		sb.WriteString("notrace:[")
		sb.WriteString(strings.Join(m.noTrace, " "))
		sb.WriteString("]\n")
	}

	// environment
	n := len(m.environment)
	sb.WriteString("environment:(")
	for k, v := range m.environment {
		n = n - 1
		sb.WriteString(k)
		sb.WriteRune(':')
		sb.WriteString(escapeValue(v))
		if n > 0 {
			sb.WriteRune(' ')
		}
	}
	sb.WriteString(")\n")

	// mounts
	if len(m.mounts) > 0 {
		sb.WriteString("mounts:(\n")
		for k, v := range m.mounts {
			sb.WriteString("    ")
			sb.WriteString(k)
			sb.WriteRune(':')
			sb.WriteString(v)
			sb.WriteRune('\n')
		}
		sb.WriteString(")\n")
	}

	if m.networkConfig != nil {
		sb.WriteString("ipaddr:")
		sb.WriteString(m.networkConfig.IP)
		sb.WriteString("\ngateway:")
		sb.WriteString(m.networkConfig.Gateway)
		sb.WriteString("\nnetmask:")
		sb.WriteString(m.networkConfig.NetMask)
		sb.WriteRune('\n')
	}

	//
	sb.WriteString(")\n")
	return sb.String()
}

func toString(m *map[string]interface{}, sb *strings.Builder, indent int) {
	for k, v := range *m {
		sb.WriteString(strings.Repeat(" ", indent))

		nvalue, nok := v.(link)
		// link
		if nok {
			sb.WriteString(escapeValue(k))
			sb.WriteString(":(linktarget:")
			sb.WriteString(escapeValue(nvalue.path))
			sb.WriteString(")\n")
			continue
		}

		value, ok := v.(string)

		// file
		if ok {
			sb.WriteString(escapeValue(k))
			sb.WriteString(":(contents:(host:")
			sb.WriteString(escapeValue(value))
			sb.WriteString("))\n")

			// dir
		} else {
			sb.WriteString(k)
			sb.WriteString(":(children:(")
			// recur
			ch := v.(map[string]interface{})
			if len(ch) > 0 {
				sb.WriteRune('\n')
				toString(&ch, sb, indent+4)
				sb.WriteString(strings.Repeat(" ", indent))
			}

			sb.WriteString("))\n")
		}
	}
}
