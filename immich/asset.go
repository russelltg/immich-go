package immich

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/simulot/immich-go/browser"
	"github.com/simulot/immich-go/helpers/fshelper"
)

type AssetResponse struct {
	ID        string `json:"id"`
	Duplicate bool   `json:"duplicate"`
}

func formatDuration(duration time.Duration) string {
	hours := duration / time.Hour
	duration -= hours * time.Hour

	minutes := duration / time.Minute
	duration -= minutes * time.Minute

	seconds := duration / time.Second
	duration -= seconds * time.Second

	milliseconds := duration / time.Millisecond

	return fmt.Sprintf("%02d:%02d:%02d.%06d", hours, minutes, seconds, milliseconds)
}

func (ic *ImmichClient) AssetUpload(ctx context.Context, la *browser.LocalAssetFile) (AssetResponse, error) {
	var ar AssetResponse
	mtype, err := fshelper.MimeFromExt(path.Ext(la.FileName))
	if err != nil {
		return ar, err
	}

	f, err := la.Open()
	if err != nil {
		return ar, (err)
	}

	body, pw := io.Pipe()
	m := multipart.NewWriter(pw)

	go func() {
		defer func() {
			m.Close()
			pw.Close()
		}()
		s, err := f.Stat()
		if err != nil {
			return
		}
		assetType := strings.ToUpper(strings.Split(mtype[0], "/")[0])
		ext := path.Ext(la.Title)
		if strings.TrimSuffix(la.Title, ext) == "" {
			la.Title = "No Name" + ext // fix #88, #128
		}

		m.WriteField("deviceAssetId", fmt.Sprintf("%s-%d", path.Base(la.Title), s.Size()))
		m.WriteField("deviceId", ic.DeviceUUID)
		m.WriteField("assetType", assetType)
		m.WriteField("fileCreatedAt", la.DateTaken.Format(time.RFC3339))
		m.WriteField("fileModifiedAt", s.ModTime().Format(time.RFC3339))
		m.WriteField("isFavorite", myBool(la.Favorite).String())
		m.WriteField("fileExtension", ext)
		m.WriteField("duration", formatDuration(0))
		m.WriteField("isReadOnly", "false")
		// m.WriteField("isArchived", myBool(la.Archived).String()) // Not supported by the api
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
				escapeQuotes("assetData"), escapeQuotes(path.Base(la.Title))))
		h.Set("Content-Type", mtype[0])

		part, err := m.CreatePart(h)
		if err != nil {
			return
		}
		_, err = io.Copy(part, f)
		if err != nil {
			return
		}
		/*
			if la.LivePhotoData != "" {
				h.Set("Content-Disposition",
					fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
						escapeQuotes("livePhotoData"), escapeQuotes(path.Base(la.LivePhotoData))))
				h.Set("Content-Type", "application/binary")
				part, err := m.CreatePart(h)
				if err != nil {
					return
				}
				b, err := la.FSys.Open(la.LivePhotoData)
				if err != nil {
					return
				}
				defer b.Close()
				_, err = io.Copy(part, b)
				if err != nil {
					return
				}
			}
		*/

		if la.SideCar != nil {
			scName := path.Base(la.FileName) + ".xmp"
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
					escapeQuotes("sidecarData"), escapeQuotes(scName)))
			h.Set("Content-Type", "application/xml")

			part, err := m.CreatePart(h)
			if err != nil {
				return
			}
			sc, err := la.SideCar.Open(la.FSys, la.SideCar.FileName)
			if err != nil {
				return
			}
			defer sc.Close()
			_, err = io.Copy(part, sc)
			if err != nil {
				return
			}
		}
	}()

	err = ic.newServerCall(ctx, "AssetUpload").
		do(post("/asset/upload", m.FormDataContentType(), setAcceptJSON(), setBody(body)), responseJSON(&ar))

	return ar, err
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

type GetAssetOptions struct {
	UserID        string
	IsFavorite    bool
	IsArchived    bool
	WithoutThumbs bool
	Skip          string
}

func (o *GetAssetOptions) Values() url.Values {
	if o == nil {
		return url.Values{}
	}
	v := url.Values{}
	v.Add("userId", o.UserID)
	v.Add("isFavorite", myBool(o.IsFavorite).String())
	v.Add("isArchived", myBool(o.IsArchived).String())
	v.Add("withoutThumbs", myBool(o.WithoutThumbs).String())
	v.Add("skip", o.Skip)
	return v
}

// GetAllAssets get all user's assets using the paged API searchAssets
//
// It calls the server for IMAGE, VIDEO, normal item, trashed Items

func (ic *ImmichClient) GetAllAssets(ctx context.Context, opt *GetAssetOptions) ([]*Asset, error) {
	var r []*Asset

	for _, t := range []string{"IMAGE", "VIDEO", "AUDIO", "OTHER"} {
		values := opt.Values()
		values.Set("type", t)
		values.Set("withExif", "true")
		values.Set("isVisible", "true")
		values.Del("trashedBefore")
		err := ic.newServerCall(ctx, "GetAllAssets", setPaginator()).do(get("/assets", setURLValues(values), setAcceptJSON()), responseAccumulateJSON(&r))
		if err != nil {
			return r, err
		}
		values.Set("trashedBefore", "9999-01-01")
		err = ic.newServerCall(ctx, "GetAllAssets", setPaginator()).do(get("/assets", setURLValues(values), setAcceptJSON()), responseAccumulateJSON(&r))
		if err != nil {
			return r, err
		}
	}
	return r, nil
}

// GetAllAssetsWithFilter get all user's assets using the paged API searchAssets and apply a filter
// TODO: rename this function, it's not a filter, it uses a callback function for each item
//
// It calls the server for IMAGE, VIDEO, normal item, trashed Items
func (ic *ImmichClient) GetAllAssetsWithFilter(ctx context.Context, opt *GetAssetOptions, filter func(*Asset)) error {
	for _, t := range []string{"IMAGE", "VIDEO", "AUDIO", "OTHER"} {
		values := opt.Values()
		values.Set("type", t)
		values.Set("withExif", "true")
		values.Set("isVisible", "true")
		values.Del("trashedBefore")
		err := ic.newServerCall(ctx, "GetAllAssets", setPaginator()).do(get("/assets", setURLValues(values), setAcceptJSON()), responseJSONWithFilter(filter))
		if err != nil {
			return err
		}
		values.Set("trashedBefore", "9999-01-01")
		err = ic.newServerCall(ctx, "GetAllAssets", setPaginator()).do(get("/assets", setURLValues(values), setAcceptJSON()), responseJSONWithFilter(filter))
		if err != nil {
			return err
		}
	}

	return nil
}

func (ic *ImmichClient) DeleteAssets(ctx context.Context, id []string, forceDelete bool) error {
	req := struct {
		Force bool     `json:"force"`
		IDs   []string `json:"ids"`
	}{
		IDs:   id,
		Force: forceDelete,
	}

	return ic.newServerCall(ctx, "DeleteAsset").do(deleteItem("/asset", setAcceptJSON(), setJSONBody(req)))
}

func (ic *ImmichClient) GetAssetByID(ctx context.Context, id string) (*Asset, error) {
	r := Asset{}
	err := ic.newServerCall(ctx, "GetAssetByID").do(get("/asset/assetById/"+id, setAcceptJSON()), responseJSON(&r))
	return &r, err
}

func (ic *ImmichClient) UpdateAssets(ctx context.Context, ids []string,
	isArchived bool, isFavorite bool,
	latitude float64, longitude float64,
	removeParent bool, stackParentID string,
) error {
	type updAssets struct {
		IDs           []string `json:"ids"`
		IsArchived    bool     `json:"isArchived"`
		IsFavorite    bool     `json:"isFavorite"`
		Latitude      float64  `json:"latitude"`
		Longitude     float64  `json:"longitude"`
		RemoveParent  bool     `json:"removeParent"`
		StackParentID string   `json:"stackParentId,omitempty"`
	}

	param := updAssets{
		IDs:           ids,
		IsArchived:    isArchived,
		IsFavorite:    isFavorite,
		Latitude:      latitude,
		Longitude:     longitude,
		RemoveParent:  removeParent,
		StackParentID: stackParentID,
	}
	return ic.newServerCall(ctx, "updateAssets").do(put("/asset", setJSONBody(param)))
}

func (ic *ImmichClient) UpdateAsset(ctx context.Context, id string, a *browser.LocalAssetFile) (*Asset, error) {
	type updAsset struct {
		IsArchived  bool    `json:"isArchived"`
		IsFavorite  bool    `json:"isFavorite"`
		Latitude    float64 `json:"latitude,omitempty"`
		Longitude   float64 `json:"longitude,omitempty"`
		Description string  `json:"description,omitempty"`
	}
	param := updAsset{
		Description: a.Description,
		IsArchived:  a.Archived,
		IsFavorite:  a.Favorite,
		Latitude:    a.Latitude,
		Longitude:   a.Longitude,
	}
	r := Asset{}
	err := ic.newServerCall(ctx, "updateAsset").do(put("/asset/"+id, setJSONBody(param)), responseJSON(&r))
	return &r, err
}

func (ic *ImmichClient) StackAssets(ctx context.Context, coverID string, ids []string) error {
	cover, err := ic.GetAssetByID(ctx, coverID)
	if err != nil {
		return err
	}

	return ic.UpdateAssets(ctx, ids, cover.IsArchived, cover.IsFavorite, cover.ExifInfo.Latitude, cover.ExifInfo.Longitude, false, coverID)
}
