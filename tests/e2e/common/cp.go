/**
# Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package common

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/exec"
)

// CopyOptions have the data required to perform the copy operation
type CopyOptions struct {
	Container  string
	Namespace  string
	NoPreserve bool
	MaxTries   int

	ClientConfig      *restclient.Config
	Clientset         kubernetes.Interface
	ExecParentCmdName string

	genericiooptions.IOStreams
}

// NewCopyOptions creates the options for copy
func NewCopyOptions(ioStreams genericiooptions.IOStreams) *CopyOptions {
	return &CopyOptions{
		IOStreams: ioStreams,
	}
}

func (o *CopyOptions) copyFromPod(src, dest fileSpec) error {
	reader := newTarPipe(src, o)
	srcFile := src.File.(remotePath)
	destFile := dest.File.(localPath)
	// remove extraneous path shortcuts - these could occur if a path contained extra "../"
	// and attempted to navigate beyond "/" in a remote filesystem
	prefix := stripPathShortcuts(srcFile.StripSlashes().Clean().String())
	return o.untarAll(src.PodNamespace, src.PodName, prefix, srcFile, destFile, reader)
}

type TarPipe struct {
	src       fileSpec
	o         *CopyOptions
	reader    *io.PipeReader
	outStream *io.PipeWriter
	bytesRead uint64
	retries   int
}

func newTarPipe(src fileSpec, o *CopyOptions) *TarPipe {
	t := new(TarPipe)
	t.src = src
	t.o = o
	t.initReadFrom(0)
	return t
}

func (t *TarPipe) initReadFrom(n uint64) {
	t.reader, t.outStream = io.Pipe()
	options := &exec.ExecOptions{
		StreamOptions: exec.StreamOptions{
			IOStreams: genericiooptions.IOStreams{
				In:     nil,
				Out:    t.outStream,
				ErrOut: t.o.Out,
			},

			Namespace: t.src.PodNamespace,
			PodName:   t.src.PodName,
		},

		Command:  []string{"tar", "cf", "-", t.src.File.String()},
		Executor: &exec.DefaultRemoteExecutor{},
	}
	if t.o.MaxTries != 0 {
		options.Command = []string{"sh", "-c", fmt.Sprintf("tar cf - %s | tail -c+%d", t.src.File, n)}
	}

	go func() {
		defer t.outStream.Close()
		if err := t.o.execute(options); err != nil {
			t.outStream.CloseWithError(err)
		}
	}()
}

func (t *TarPipe) Read(p []byte) (n int, err error) {
	n, err = t.reader.Read(p)
	if err != nil {
		if t.o.MaxTries < 0 || t.retries < t.o.MaxTries {
			t.retries++
			fmt.Printf("Resuming copy at %d bytes, retry %d/%d\n", t.bytesRead, t.retries, t.o.MaxTries)
			t.initReadFrom(t.bytesRead + 1)
			err = nil
		} else {
			fmt.Printf("Dropping out copy after %d retries\n", t.retries)
		}
	} else {
		t.bytesRead += uint64(n)
	}
	return
}

func (o *CopyOptions) untarAll(ns, pod string, prefix string, src remotePath, dest localPath, reader io.Reader) error {
	symlinkWarningPrinted := false
	// TODO: use compression here?
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		// All the files will start with the prefix, which is the directory where
		// they were located on the pod, we need to strip down that prefix, but
		// if the prefix is missing it means the tar was tempered with.
		// For the case where prefix is empty we need to ensure that the path
		// is not absolute, which also indicates the tar file was tempered with.
		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("tar contents corrupted")
		}

		// basic file information
		mode := header.FileInfo().Mode()
		// header.Name is a name of the REMOTE file, so we need to create
		// a remotePath so that it goes through appropriate processing related
		// with cleaning remote paths
		destFileName := dest.Join(newRemotePath(header.Name[len(prefix):]))

		if !isRelative(dest, destFileName) {
			fmt.Fprintf(o.IOStreams.ErrOut, "warning: file %q is outside target destination, skipping\n", destFileName)
			continue
		}

		if err := os.MkdirAll(destFileName.Dir().String(), 0755); err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(destFileName.String(), 0755); err != nil {
				return err
			}
			continue
		}

		if mode&os.ModeSymlink != 0 {
			if !symlinkWarningPrinted && len(o.ExecParentCmdName) > 0 {
				fmt.Fprintf(o.IOStreams.ErrOut,
					"warning: skipping symlink: %q -> %q (consider using \"%s exec -n %q %q -- tar cf - %q | tar xf -\")\n",
					destFileName, header.Linkname, o.ExecParentCmdName, ns, pod, src)
				symlinkWarningPrinted = true
				continue
			}
			fmt.Fprintf(o.IOStreams.ErrOut, "warning: skipping symlink: %q -> %q\n", destFileName, header.Linkname)
			continue
		}
		outFile, err := os.Create(destFileName.String())
		if err != nil {
			return err
		}
		defer outFile.Close()
		if _, err := io.Copy(outFile, tarReader); err != nil {
			return err
		}
		if err := outFile.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (o *CopyOptions) execute(options *exec.ExecOptions) error {
	if len(options.Namespace) == 0 {
		options.Namespace = o.Namespace
	}

	if len(o.Container) > 0 {
		options.ContainerName = o.Container
	}

	options.Config = o.ClientConfig
	options.PodClient = o.Clientset.CoreV1()

	if err := options.Validate(); err != nil {
		return err
	}

	return options.Run()
}

type fileSpec struct {
	PodName      string
	PodNamespace string
	File         pathSpec
}

type pathSpec interface {
	String() string
}

// localPath represents a client-native path, which will differ based
// on the client OS, its methods will use path/filepath package which
// is OS dependant
type localPath struct {
	file string
}

func newLocalPath(fileName string) localPath {
	file := stripTrailingSlash(fileName)
	return localPath{file: file}
}

func (p localPath) String() string {
	return p.file
}

func (p localPath) Dir() localPath {
	return newLocalPath(filepath.Dir(p.file))
}

func (p localPath) Base() localPath {
	return newLocalPath(filepath.Base(p.file))
}

func (p localPath) Clean() localPath {
	return newLocalPath(filepath.Clean(p.file))
}

func (p localPath) Join(elem pathSpec) localPath {
	return newLocalPath(filepath.Join(p.file, elem.String()))
}

func (p localPath) Glob() (matches []string, err error) {
	return filepath.Glob(p.file)
}

func (p localPath) StripSlashes() localPath {
	return newLocalPath(stripLeadingSlash(p.file))
}

func isRelative(base, target localPath) bool {
	relative, err := filepath.Rel(base.String(), target.String())
	if err != nil {
		return false
	}
	return relative == "." || relative == stripPathShortcuts(relative)
}

// remotePath represents always UNIX path, its methods will use path
// package which is always using `/`
type remotePath struct {
	file string
}

func newRemotePath(fileName string) remotePath {
	// we assume remote file is a linux container but we need to convert
	// windows path separators to unix style for consistent processing
	file := strings.ReplaceAll(stripTrailingSlash(fileName), `\`, "/")
	return remotePath{file: file}
}

func (p remotePath) String() string {
	return p.file
}

func (p remotePath) Dir() remotePath {
	return newRemotePath(path.Dir(p.file))
}

func (p remotePath) Base() remotePath {
	return newRemotePath(path.Base(p.file))
}

func (p remotePath) Clean() remotePath {
	return newRemotePath(path.Clean(p.file))
}

func (p remotePath) Join(elem pathSpec) remotePath {
	return newRemotePath(path.Join(p.file, elem.String()))
}

func (p remotePath) StripShortcuts() remotePath {
	p = p.Clean()
	return newRemotePath(stripPathShortcuts(p.file))
}

func (p remotePath) StripSlashes() remotePath {
	return newRemotePath(stripLeadingSlash(p.file))
}

// strips trailing slash (if any) both unix and windows style
func stripTrailingSlash(file string) string {
	if len(file) == 0 {
		return file
	}
	if file != "/" && strings.HasSuffix(string(file[len(file)-1]), "/") {
		return file[:len(file)-1]
	}
	return file
}

func stripLeadingSlash(file string) string {
	// tar strips the leading '/' and '\' if it's there, so we will too
	return strings.TrimLeft(file, `/\`)
}

// stripPathShortcuts removes any leading or trailing "../" from a given path
func stripPathShortcuts(p string) string {
	newPath := p
	trimmed := strings.TrimPrefix(newPath, "../")

	for trimmed != newPath {
		newPath = trimmed
		trimmed = strings.TrimPrefix(newPath, "../")
	}

	// trim leftover {".", ".."}
	if newPath == "." || newPath == ".." {
		newPath = ""
	}

	if len(newPath) > 0 && string(newPath[0]) == "/" {
		return newPath[1:]
	}

	return newPath
}

var (
	errFileSpecDoesntMatchFormat = errors.New("filespec must match the canonical format: [[namespace/]pod:]file/path")
)

func extractFileSpec(arg string) (fileSpec, error) {
	i := strings.Index(arg, ":")

	// filespec starting with a semicolon is invalid
	if i == 0 {
		return fileSpec{}, errFileSpecDoesntMatchFormat
	}
	if i == -1 {
		return fileSpec{
			File: newLocalPath(arg),
		}, nil
	}

	pod, file := arg[:i], arg[i+1:]
	pieces := strings.Split(pod, "/")
	switch len(pieces) {
	case 1:
		return fileSpec{
			PodName: pieces[0],
			File:    newRemotePath(file),
		}, nil
	case 2:
		return fileSpec{
			PodNamespace: pieces[0],
			PodName:      pieces[1],
			File:         newRemotePath(file),
		}, nil
	default:
		return fileSpec{}, errFileSpecDoesntMatchFormat
	}
}
