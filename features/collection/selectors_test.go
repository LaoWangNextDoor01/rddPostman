package collection

import (
	"reflect"
	"testing"

	"rddpostman/internal/domain"
)

func TestNodeTrail(t *testing.T) {
	// 构造多级层级结构：
	// 集合 (coll-1: "开放平台")
	//   └─ 目录 1 (dir-1: "v1")
	//        └─ 目录 2 (dir-2: "auth")
	//             └─ 请求 (req-1: "登录")
	//   └─ 目录 3 (dir-3: "v2")
	collections := []*domain.Collection{
		{
			ID:   "coll-1",
			Name: "开放平台",
			Items: []*domain.Node{
				{
					ID:   "dir-1",
					Name: "v1",
					Kind: domain.KindFolder,
					Children: []*domain.Node{
						{
							ID:   "dir-2",
							Name: "auth",
							Kind: domain.KindFolder,
							Children: []*domain.Node{
								{
									ID:   "req-1",
									Name: "登录",
									Kind: domain.KindRequest,
								},
							},
						},
					},
				},
				{
					ID:   "dir-3",
					Name: "v2",
					Kind: domain.KindFolder,
				},
			},
		},
	}

	// 1. 测试根集合
	trailColl := nodeTrail(collections, "coll-1")
	if len(trailColl) != 1 || trailColl[0].Name != "开放平台" {
		t.Fatalf("集合根路径不匹配: %+v", trailColl)
	}

	// 2. 测试二级：集合 -> 目录 1
	trailDir1 := nodeTrail(collections, "dir-1")
	expected1 := []string{"开放平台", "v1"}
	if !reflect.DeepEqual(names(trailDir1), expected1) {
		t.Fatalf("二级路径不匹配: got %v, want %v", names(trailDir1), expected1)
	}

	// 3. 测试三级：集合 -> 目录 1 -> 目录 2（用户反馈的关键场景）
	trailDir2 := nodeTrail(collections, "dir-2")
	expected2 := []string{"开放平台", "v1", "auth"}
	if !reflect.DeepEqual(names(trailDir2), expected2) {
		t.Fatalf("三级目录路径不匹配（中间目录被漏掉）: got %v, want %v", names(trailDir2), expected2)
	}

	// 4. 测试四级：集合 -> 目录 1 -> 目录 2 -> 请求
	trailReq := nodeTrail(collections, "req-1")
	expectedReq := []string{"开放平台", "v1", "auth", "登录"}
	if !reflect.DeepEqual(names(trailReq), expectedReq) {
		t.Fatalf("四级路径不匹配: got %v, want %v", names(trailReq), expectedReq)
	}

	// 5. 测试同级目录 v2
	trailDir3 := nodeTrail(collections, "dir-3")
	expected3 := []string{"开放平台", "v2"}
	if !reflect.DeepEqual(names(trailDir3), expected3) {
		t.Fatalf("同级目录路径不匹配: got %v, want %v", names(trailDir3), expected3)
	}

	// 6. 验证 nodePath 格式化字符串
	pathReq := nodePath(collections, "req-1")
	if pathReq != "开放平台 / v1 / auth / 登录" {
		t.Fatalf("nodePath 不匹配: got %q", pathReq)
	}
}

func names(crumbs []CrumbItem) []string {
	res := make([]string, len(crumbs))
	for i, c := range crumbs {
		res[i] = c.Name
	}
	return res
}
