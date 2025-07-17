package internal

import (
	"go/ast"

	"github.com/vektra/mockery/v3/config"
	"golang.org/x/tools/go/packages"
)

type Interface struct {
	Name       string // Name of the type to be mocked.
	TypeSpec   *ast.TypeSpec
	GenDecl    *ast.GenDecl
	FilePath   string
	FileSyntax *ast.File
	Pkg        *packages.Package
	Config     *config.Config
}

func NewInterface(
	name string,
	typeSpec *ast.TypeSpec,
	genDecl *ast.GenDecl,
	filepath string,
	fileSyntax *ast.File,
	pkg *packages.Package,
	config *config.Config,
) *Interface {
	return &Interface{
		Name:       name,
		TypeSpec:   typeSpec,
		GenDecl:    genDecl,
		FilePath:   filepath,
		FileSyntax: fileSyntax,
		Pkg:        pkg,
		Config:     config,
	}
}
