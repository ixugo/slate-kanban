package kanban

// Label 是看板标签库里的一项：给标签起名字、配颜色。
// 卡片上存的仍然是标签名（Card.Tags），颜色靠名称到标签库里查；
// 名称不在标签库里的标签照常使用，只是没有颜色。ID 是库内的稳定编号，
// 靠它才能分清"改名"和"删除再新建"。
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// LabelInput 是修改标签库时的一项入参。ID 为空表示新增，由服务端分配；
// 非空必须是库里已有的编号，名称不同于原来就是改名。
type LabelInput struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
