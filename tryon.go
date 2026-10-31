package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// The local preflight is a hard gate for Wardrobe-managed try-ons.
// It does not control independent image generators invoked outside Wardrobe.
// User-confirmed provenance is explicitly distinguished from computer vision verification.
type tryonEvidenceInput struct {
	LookID           string `json:"lookId"`
	Role             string `json:"role"` // face or body
	ConfirmRealPhoto bool   `json:"confirmRealPhoto"`
	ConfirmFrontal   bool   `json:"confirmFrontal,omitempty"`
	ConfirmFullBody  bool   `json:"confirmFullBody,omitempty"`
}
type tryonPreflightInput struct {
	LookID     string `json:"lookId"`
	FaceLookID string `json:"faceLookId"`
	BodyLookID string `json:"bodyLookId"`
}
type tryonPreflightOutput struct {
	Ready                   bool              `json:"ready"`
	Stage                   string            `json:"stage"`
	Blocks                  []string          `json:"blocks"`
	LookID                  string            `json:"lookId"`
	FaceLookID              string            `json:"faceLookId"`
	BodyLookID              string            `json:"bodyLookId"`
	Garments                map[string]string `json:"garments"`
	Accessories             map[string]string `json:"accessories"`
	HairID                  string            `json:"hairId,omitempty"`
	Pose                    string            `json:"pose"`
	Output                  string            `json:"output"`
	LookbookLayout          string            `json:"lookbookLayout"`
	IdentityReferenceDigest string            `json:"identityReferenceDigest,omitempty"`
	BodyReferenceDigest     string            `json:"bodyReferenceDigest,omitempty"`
}

// These keys are fixed in code, not delegated to model instructions.
var tryonGarmentSlots = map[string]bool{"outer": true, "mid": true, "inner": true, "pants": true, "shoes": true, "socks": true}
var tryonAccessorySlots = map[string]bool{"bag": true, "watch": true, "glasses": true, "belt": true, "hat": true, "gloves": true, "scarf": true, "tie": true, "necklace": true, "accessory1": true, "accessory2": true, "other": true}

func tryonReadJSON(r *http.Request, dst any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("single JSON object required")
	}
	return nil
}

func (a *application) tryonLookPhoto(lookID string) (string, error) {
	if !validID.MatchString(lookID) {
		return "", errors.New("invalid photo look id")
	}
	look, e := a.s.get("looks", lookID)
	if e != nil {
		return "", fmt.Errorf("look photo not available")
	}
	path, ok := look["photo"].(string)
	if !ok || path != "/media/photos/"+lookID+".jpg" {
		return "", errors.New("look has no canonical archived JPEG photo")
	}
	filename := filepath.Join(a.s.dir, "photos", lookID+".jpg")
	stat, e := os.Lstat(filename)
	if e != nil || !stat.Mode().IsRegular() {
		return "", errors.New("archived photo missing or not a regular file")
	}
	if stat.Size() < 1000 || stat.Size() > 5<<20 {
		return "", errors.New("photo size outside allowed limits")
	}
	raw, e := os.ReadFile(filename)
	if e != nil {
		return "", e
	}
	cfg, e := jpeg.DecodeConfig(bytes.NewReader(raw))
	if e != nil || cfg.Width < 256 || cfg.Height < 256 {
		return "", errors.New("photo is not a valid sufficient-resolution JPEG")
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func (a *application) verifyTryonEvidence(in tryonEvidenceInput, sources ...string) error {
	if !validID.MatchString(in.LookID) || (in.Role != "face" && in.Role != "body") {
		return errors.New("invalid look or role")
	}
	if !in.ConfirmRealPhoto {
		return errors.New("user must confirm this is a real photo, not AI-generated")
	}
	if in.Role == "face" && !in.ConfirmFrontal {
		return errors.New("user must confirm unobstructed frontal face")
	}
	if in.Role == "body" && !in.ConfirmFullBody {
		return errors.New("user must confirm a real full-body photo")
	}
	hash, e := a.tryonLookPhoto(in.LookID)
	if e != nil {
		return e
	}
	// Photo proof is tied to its content digest. Replacing the photo invalidates proof.
	id := "EVID-" + in.LookID
	existing, e := a.s.get("tryon_evidence", id)
	version := 0
	if e != nil {
		if !isNoRows(e) {
			return e
		}
		existing = record{"lookId": in.LookID}
	} else {
		version = existing["version"].(int)
		if existing["photoSHA256"] != hash {
			// New image means earlier evidence is revoked.
			delete(existing, "faceConfirmed")
			delete(existing, "bodyConfirmed")
		}
	}
	existing["photoSHA256"] = hash
	existing["confirmedByUser"] = true
	source := "local-ui-user-declaration-not-facial-recognition"
	if len(sources) > 0 {
		source = sources[0]
	}
	existing["verificationMethod"] = source
	existing["confirmedAt"] = time.Now().UTC().Format(time.RFC3339)
	if in.Role == "face" {
		existing["faceConfirmed"] = true
	} else {
		existing["bodyConfirmed"] = true
	}
	_, e = a.s.save("tryon_evidence", id, existing, version)
	return e
}

func (a *application) confirmedTryonPhoto(lookID, role string) (string, error) {
	hash, e := a.tryonLookPhoto(lookID)
	if e != nil {
		return "", e
	}
	proof, e := a.s.get("tryon_evidence", "EVID-"+lookID)
	if e != nil {
		return "", errors.New("photo has not been explicitly confirmed by the user")
	}
	if proof["photoSHA256"] != hash || proof["confirmedByUser"] != true || proof[role+"Confirmed"] != true {
		return "", errors.New("photo provenance / role confirmation missing or photo changed")
	}
	return hash, nil
}

func (a *application) preflightTryon(in tryonPreflightInput) tryonPreflightOutput {
	o := tryonPreflightOutput{
		Stage: "blocked", Blocks: []string{}, LookID: in.LookID, FaceLookID: in.FaceLookID, BodyLookID: in.BodyLookID,
		Garments: map[string]string{}, Accessories: map[string]string{},
		Pose:           "single-person, frontal face and torso, head upright, eyes to camera, arms relaxed, hands empty, feet parallel, even weight, no forward leg or head tilt",
		Output:         "AI stage: one full-length realistic photograph only; no generated sidebar, labels, captions or extra accessories. Final lookbook sidebar is drawn locally by Wardrobe Canvas.",
		LookbookLayout: "lookbook_card_v1",
	}
	block := func(s string) { o.Blocks = append(o.Blocks, s) }
	if !validID.MatchString(in.LookID) {
		block("invalid outfit ID")
	} else {
		look, e := a.s.get("looks", in.LookID)
		if e != nil {
			block("outfit not found")
		} else {
			slots, ok := look["slots"].(map[string]any)
			if !ok {
				block("outfit slots missing")
			} else {
				for slot, value := range slots {
					id, ok := value.(string)
					if !ok {
						block("invalid outfit slot " + slot)
						continue
					}
					if id == "" {
						continue
					}
					if !validID.MatchString(id) {
						block("invalid item ID")
						continue
					}
					if slot == "hair" {
						_, e := a.s.get("hairstyles", id)
						if e != nil {
							block("hairstyle reference missing")
						} else {
							o.HairID = id
						}
						continue
					}
					if !tryonGarmentSlots[slot] && !tryonAccessorySlots[slot] {
						block("unknown outfit slot " + slot)
						continue
					}
					p, e := a.s.get("products", id)
					if e != nil {
						block("product for " + slot + " not found")
						continue
					}
					if tryonGarmentSlots[slot] {
						o.Garments[slot] = id
					} else {
						o.Accessories[slot] = id
					}
					expected := "/media/images/" + id + ".jpg"
					if p["image"] != expected {
						block("verified product image missing for " + slot)
						continue
					}
					path := filepath.Join(a.s.dir, "images", id+".jpg")
					fi, e := os.Lstat(path)
					if e != nil || !fi.Mode().IsRegular() || fi.Size() < 1000 || fi.Size() > 5<<20 {
						block("valid product image file missing for " + slot)
						continue
					}
					data, e := os.ReadFile(path)
					if e != nil {
						block("cannot read product image for " + slot)
						continue
					}
					if _, e := jpeg.DecodeConfig(bytes.NewReader(data)); e != nil {
						block("invalid product JPEG for " + slot)
					}
				}
				if len(o.Garments) < 2 {
					block("at least two actual garment slots required")
				}
			}
		}
	}
	if in.FaceLookID == "" {
		block("explicit frontal real-person identity photo required")
	} else {
		hash, e := a.confirmedTryonPhoto(in.FaceLookID, "face")
		if e != nil {
			block("identity photo: " + e.Error())
		} else {
			o.IdentityReferenceDigest = hash
		}
	}
	if in.BodyLookID == "" {
		block("explicit full-body real-person photo required")
	} else {
		hash, e := a.confirmedTryonPhoto(in.BodyLookID, "body")
		if e != nil {
			block("body photo: " + e.Error())
		} else {
			o.BodyReferenceDigest = hash
		}
	}
	sort.Strings(o.Blocks)
	if len(o.Blocks) == 0 {
		o.Ready = true
		o.Stage = "ready_for_renderer_only"
	}
	return o
}

// A controlled renderer must receive image bytes, not just the textual plan.
// No renderer is configured in this alpha; generation MUST fail closed.
func (a *application) serveTryon(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/api/tryon/confirm-photo":
		var in tryonEvidenceInput
		if e := tryonReadJSON(r, &in); e != nil {
			fail(w, 400, e)
			return
		}
		if e := a.verifyTryonEvidence(in); e != nil {
			fail(w, 400, e)
			return
		}
		jsonOut(w, 200, map[string]any{"saved": true, "lookId": in.LookID, "role": in.Role, "notice": "user-declared provenance, not biometric verification"})
	case "/api/tryon/preflight":
		var in tryonPreflightInput
		if e := tryonReadJSON(r, &in); e != nil {
			fail(w, 400, e)
			return
		}
		jsonOut(w, 200, a.preflightTryon(in))
	case "/api/tryon/render":
		// Do not let a language model or HTTP caller bypass photo verification.
		// This endpoint cannot call an unverified provider and cannot save a Look.
		http.Error(w, "render unavailable: no image-input-verified renderer and output validator configured", http.StatusNotImplemented)
	default:
		http.NotFound(w, r)
	}
}
