package template

import "go/ast"

type Comments struct {
	/*
		GenDeclDoc represents the doc comments for a general declaration.
		For example, if you were to define an interface with the following comments:

		// Foo defines Bar
		type Foo interface {
			Bar() string
		}

		then GenDeclDoc will contain the "// Foo defines Bar" comment.
		Similarly, if you define your interface like:

		// hello world
		type (
			// Foo defines Bar
			Foo interface {
				Bar() string
			}
		)

		then GenDeclDoc will contain the "// hello world" comment.
	*/
	GenDeclDoc CommentGroup
	/*
		TypeSpecComment contains in-line comments for a type spec.
		For example:

		type Foo interface {
			Bar() string
		} // This is a line comment
	*/
	TypeSpecComment CommentGroup

	/*
		TypeSpecDoc contains the docs for a type spec.
		For example:

		type (
			// Foo defines Bar
			Foo interface {
				Bar() string
			}
		)

		TypeSpecDoc will _not_ contain the comments defined like this:

		// Foo defines Bar
		type Foo interface {
			Bar() string
		}

		The reason is because the Go AST defines this as a comment on an *ast.GenDecl, not an *ast.TypeSpec.
	*/
	TypeSpecDoc CommentGroup
}

func NewComments(typeSpec *ast.TypeSpec, genDecl *ast.GenDecl) Comments {
	return Comments{
		GenDeclDoc:      NewCommentGroupFromAST(genDecl.Doc),
		TypeSpecComment: NewCommentGroupFromAST(typeSpec.Comment),
		TypeSpecDoc:     NewCommentGroupFromAST(typeSpec.Doc),
	}
}
