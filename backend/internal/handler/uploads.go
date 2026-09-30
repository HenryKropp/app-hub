package handler

type uploadHub struct{}

func newUploadHub() *uploadHub        { return &uploadHub{} }
func (u *uploadHub) missing(id int64) {}
