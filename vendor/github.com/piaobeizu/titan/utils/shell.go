/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2021/2021/04 04/46/50
 @Desc    :
*/

package utils

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

func ExecLinuxCmd(command string, args []string, prefix string) (err error) {
	//函数返回一个*Cmd，用于使用给出的参数执行name指定的程序
	// cmd := exec.Command(command, params...)
	cmd := exec.Command(command, args...)
	//显示运行的命令
	log.Infof("[cmd] %s", strings.Join(cmd.Args, " "))

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err = cmd.Start(); err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	defer wg.Wait()
	wg.Add(2)
	go readLog(&wg, stdout, prefix)
	go readLog(&wg, stderr, prefix)
	// go copyAndCapture(os.Stdout, stdout)
	// go copyAndCapture(os.Stderr, stderr)
	if err = cmd.Wait(); err != nil {
		return
	}
	return
}

func ExecLinuxCmdWithOutput(command string, args []string) (output string, err error) {
	//函数返回一个*Cmd，用于使用给出的参数执行name指定的程序
	// cmd := exec.Command(command, params...)
	cmd := exec.Command(command, args...)
	//显示运行的命令
	log.Infof("exec cmd is %s", cmd.Args)

	stdout, _ := cmd.StdoutPipe()
	if err = cmd.Start(); err != nil {
		return
	}
	result, _ := ioutil.ReadAll(stdout) // 读取输出结果
	output = string(result)

	if err = cmd.Wait(); err != nil {
		return
	}
	return
}

func readLog(wg *sync.WaitGroup, reader io.ReadCloser, prefix string) {
	defer func() {
		if r := recover(); r != nil {
			log.Info(r, string(debug.Stack()))
		}
	}()
	defer wg.Done()
	r := bufio.NewReader(reader)
	for {
		line, _, err := r.ReadLine()
		if err == io.EOF || err != nil {
			return
		}
		linestr := string(line[:])
		fmt.Println(linestr)
		// linestr = strings.Replace(linestr, "==> ", "", -1)
		// if strings.Trim(linestr, " ") != "" {
		// 	level, linestr := strings.ToUpper(linestr), prefix+linestr
		// 	if strings.Contains(level, "[DEBUG") || strings.Contains(level, "[DEBG]") {
		// 		log.Debug(linestr)
		// 	} else if strings.Contains(level, "[WARN") {
		// 		log.Warning(linestr)
		// 	} else if strings.Contains(level, "[EROR") || strings.Contains(level, "[EROR]") {
		// 		log.Error(linestr)
		// 	} else if strings.Contains(level, "[PANIC") || strings.Contains(level, "[PANI]") {
		// 		log.Panic(linestr)
		// 	} else if strings.Contains(level, "[FATAL") || strings.Contains(level, "[FATA]") {
		// 		log.Fatal(linestr)
		// 	} else {
		// 		if len(linestr) > 0 {
		// 			log.Info(linestr)
		// 		}
		// 	}
		// }
	}
}

func copyAndCapture(w io.Writer, r io.Reader) ([]byte, error) {
	var out []byte
	buf := make([]byte, 1024, 1024)
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			d := buf[:n]
			out = append(out, d...)
			os.Stdout.Write(d)
		}
		if err != nil {
			// Read returns io.EOF at the end of file, which is not an error for us
			if err == io.EOF {
				err = nil
			}
			return out, err
		}
	}
	// never reached
	// panic(true)
	// return nil, nil
}
