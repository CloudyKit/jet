package jet

import (
	"bytes"
	"io"
)

// A transaction owns only its temporary output. The render keeps the same
// execution state through nested buffering, rollback, and commit.
type outputTransaction struct {
	parent io.Writer
	buffer bytes.Buffer
}

func (st *Runtime) beginOutputTransaction() *outputTransaction {
	st.checkpoint(0)
	tx := &outputTransaction{parent: st.Writer}
	st.Writer = &tx.buffer
	return tx
}

func (st *Runtime) commitOutputTransaction(tx *outputTransaction) {
	st.checkpoint(0)
	st.Writer = tx.parent
	if _, err := io.Copy(tx.parent, &tx.buffer); err != nil {
		st.terminate(err)
	}
	st.checkpoint(0)
}
