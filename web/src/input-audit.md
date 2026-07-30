# Editable field remount audit

Audited all editable/focusable fields and their keyed ancestor chains under `web/src`.

- `ChannelModelEditor`: selected model rows are the only mapped editable rows. They now use immutable `ui_id`; both text fields and the row checkbox remain under that stable row.
- `Channels`, `Keys`, `Presets`, `Settings`, `Capture`, `Logs`, `Login`, and `Setup`: form fields are not inside value-keyed mapped ancestors. Conditional form wrappers have stable component types and are not keyed by edited values.
- Mapped `<option>` elements in `Keys` use database IDs; changing the select value does not change option identity.
- Read-only capture key and settings checkboxes have stable, unkeyed ancestor chains.

Regression tests exercise both editable selected-model text fields. No other remount-on-edit pattern was found.
