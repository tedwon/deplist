package scan

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type mvnString string

func gatherMvn(mvn string) (string, string, error) {
	mvnStr := mvn
	// remove the :compile or :runtime off the end
	lastColon := strings.LastIndex(mvnStr, ":")
	if lastColon == -1 {
		return "", "", fmt.Errorf("Invalid maving parsing, looking for ':'")
	}
	mvnStr = mvnStr[:lastColon]

	mvn = strings.Replace(mvn, ":compile", "", 1)
	mvn = strings.Replace(mvn, ":provided", "", 1)

	verIdx := strings.LastIndex(mvn, ":")
	if verIdx == -1 || len(mvn) < (verIdx+1) {
		return "", "", fmt.Errorf("Invalid maving parsing, looking for version ':'")
	}
	ver := mvn[verIdx+1:]

	ver = strings.TrimSuffix(ver, "\" ; ") // TOOD watch whitespace
	ver = strings.TrimSuffix(ver, " (optional) ")

	if mvnStr[0] == '"' {
		mvnStr = mvnStr[1:]
	}

	mvnStr = strings.Replace(mvnStr, ":"+ver, "", 1)
	return mvnStr, "v" + ver, nil // add "v" to version for semver compares
}

func GetMvnDeps(path string) ([][2]string, error) {
	r := regexp.MustCompile("\"(.*?)\"")
	var gathered [][2]string // array of [name, ver]string

	seen := make(map[string]struct{})

	dirPath := filepath.Dir(path)

	cmd := exec.Command("mvn",
		"--no-transfer-progress",
		"-B", // disables colors
		"dependency:tree",
		"-DskipTests",
		"-Dassembly.tarLongFileMode=posix",
		"-DfailIfNoTests=false",
		"-Dtest=false",
		"-Pdeploy",
		"-DoutputType=dot")
	cmd.Dir = dirPath

	// todo work out a better way to do this
	if _, err := os.Stat("$HOME/.m2/settings.xml"); os.IsNotExist(err) {
		cmd.Args = append(cmd.Args, "-s", filepath.Join(os.Getenv("HOME"), "/.m2/settings.xml"))
	}

	// supress error, it always returns errors
	data, _ := cmd.Output()

	res := strings.Split(string(data), "\n")

	for _, s := range res {
		// example:
		// [INFO] 	"com.google.inject:guice:jar:4.0:compile (optional) " -> "javax.inject:javax.inject:jar:1:compile (optional) " ;

		// do the lookup once
		sepIdx := strings.Index(s, "->")

		if sepIdx != -1 {
			// skip import and test
			// avoid errors downloading deps, not much we can do here
			if strings.Contains(s, ":test") || strings.Contains(s, ":import") || strings.Contains(s, "ERROR") {
				continue
			}

			// only get the second part
			part := s[sepIdx+len("-> "):]

			repo, version, err := gatherMvn(part)

			// only if no error append
			if err == nil {

				// lookup first if we have an entry
				if _, ok := seen[repo+version]; ok {
					continue
				}

				gathered = append(gathered, [2]string{repo, version})
				seen[repo+version] = struct{}{}
			}

		} else {
			if strings.Contains(s, "digraph") {
				if strings.Contains(s, ":pom:") {
					continue
				}

				s = strings.Replace(s, ":bundle", "", 1)
				var part string
				// record the package name
				res := r.FindStringSubmatch(s)
				if len(res) > 1 {
					part = res[1]
				} else {
					continue
				}
				repo, version, err := gatherMvn(part)

				// only if no error append
				if err == nil {

					// lookup first if we have an entry
					if _, ok := seen[repo+version]; ok {
						continue
					}

					gathered = append(gathered, [2]string{repo, version})
					seen[repo+version] = struct{}{}
				}
			}
		}
	}
	return gathered, nil
}
