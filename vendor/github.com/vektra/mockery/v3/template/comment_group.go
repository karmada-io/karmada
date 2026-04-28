package template

import "go/ast"

// Comment represents a single line of comments exactly as it appears in source.
// This includes the "//" or "/*" strings if present.
type Comment string

type CommentGroup struct {
	// List contains each individual line of the comments exactly as they appear
	// in source, including comment characters.
	List []Comment
	// Text contains the text of the comments without comment characters.
	Text string
}

func NewCommentGroupFromAST(comments *ast.CommentGroup) CommentGroup {
	group := CommentGroup{
		List: []Comment{},
		Text: "",
	}
	if comments == nil {
		return group
	}
	group.Text = comments.Text()
	for _, line := range comments.List {
		group.List = append(group.List, Comment(line.Text))
	}
	return group
}
