# Virtual try-on safeguards and current limitations

[简体中文](TRYON.zh-CN.md) · [Home](../README.md)

Wardrobe performs **strict, fail-closed preflight** before attempting any identity-sensitive virtual try-on. The renderer itself **has not been integrated**; the protected POST /api/tryon/render endpoint currently returns HTTP 501. Do not confuse a prepared prompt or fixed image collage with a successful, verified face-preserving try-on.

## Preflight process

~~~text
Saved look with actual SKU image files
            +
User-confirmed real frontal face photo and full-body photo
            |
User declaration + archived JPEG SHA-256
            |
preflight_wardrobe_tryon / local /api/tryon/preflight
           / \
   blocked    ready_for_renderer_only
                  |
        [controlled image renderer NOT AVAILABLE]
~~~

The local Web UI provides explicit confirmation controls. MCP get_wardrobe_tryon_evidence reads confirmation state, while confirm_wardrobe_tryon_photo may write a user-declared source/role only after the user explicitly confirms the photo is real and frontal/full-body. Changing the archived file invalidates its digest evidence.

The preflight checks actual clothing slots and JPEG files, hairstyle references, and a whitelist of explicitly selected accessories. Pose guidance requires one subject, frontal level head, both feet parallel and naturally standing, relaxed arms and no unnecessary props.

## Why rendering is disabled

No controlled renderer currently demonstrates that the reference-face bytes and the actual SKU images were fed to the generation model. There is also no reliable automated verifier for the resulting face, clothing/extra accessories or full-body posture. Until these are implemented, a successful preflight means **ready for renderer only**, not "user identity verified" or "AI try-on passed".

Independent image generation tools cannot be controlled by Wardrobe's preflight. An anonymous AI-model illustration can still be used as a **clearly labeled style preview**, followed by deterministic composition with compose_wardrobe_lookbook, but must not be saved as the user's authentic photo.

## API summary

- POST /api/tryon/confirm-photo: explicit user declaration for an archived Look photo, including face/body role.
- POST /api/tryon/preflight: structured ready/blocks information, selected SKUs, fixed pose instructions.
- POST /api/tryon/render: **HTTP 501** until the image-input provider and output validation exist.
- MCP preflight_wardrobe_tryon, get_wardrobe_tryon_evidence, confirm_wardrobe_tryon_photo: corresponding limited interfaces.
- MCP compose_wardrobe_lookbook: deterministic compositing of an independently supplied image — not AI changing clothes.

## Privacy

User-confirmed origin is **not** biometric identification. Only local image metadata and digests are recorded as evidence. If you choose an external cloud image generator, actual person images and SKU references may be sent to that provider; do not send unrelated purchases, complete inventories or secret information. See [Security](../SECURITY.md).