// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package handler

import (
	"net/http"

	"cuniBTCReward/api/internal/logic"
	"cuniBTCReward/api/internal/svc"
	"cuniBTCReward/api/internal/types"
	"cuniBTCReward/api/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ProfileLineHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ProfileLineReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewProfileLineLogic(r.Context(), svcCtx)
		resp, err := l.ProfileLine(&req)
		response.Response(w, resp, err)
	}
}
