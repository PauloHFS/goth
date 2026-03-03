# UI Components Library

This directory contains reusable UI components for the GOTH Stack.

## Components

### Layout Components

- `head.templ` - SEO meta tags, Open Graph, Twitter Cards
- `breadcrumbs.templ` - Navigation breadcrumbs with schema.org support

### Feedback Components

- `alert.templ` - Alert banners (success, error, warning, info)
- `toast.templ` - Toast notifications with auto-dismiss
- `modal.templ` - Modal dialogs with Alpine.js

### Data Display

- `table.templ` - Data tables with sorting and pagination
- `badge_card_skeleton_avatar.templ` - Badges, Cards, Skeletons, Avatars
- `tabs_accordion.templ` - Tabs and accordion components

### Form Components

- `input.templ` - Input fields, textarea, select, checkbox, radio, toggle
- `button.templ` - Buttons with variants and icons

### Navigation

- `dropdown.templ` - Dropdown menus and user menus

### Utilities

- `empty_state.templ` - Empty states for lists and search results

## Usage

Import components in your Templ files:

```go
package pages

import "github.com/PauloHFS/goth/web/components"

templ Dashboard() {
    @components.HeadComponent("Dashboard", "Manage your account", "/images/dashboard.png", "/dashboard")

    <div>
        @components.AlertComponent("success", "Welcome!", "Your dashboard is ready.")

        @components.Card("Stats", "Overview") {
            <p>Your content here</p>
        }
    </div>
}
```

## Alpine.js Integration

Components use Alpine.js for interactivity. Available stores:

- `Alpine.store('theme')` - Theme management (light/dark)
- `Alpine.store('user')` - User state
- `Alpine.store('modal')` - Modal management
- `Alpine.store('notifications')` - In-app notifications
- `Alpine.store('sidebar')` - Sidebar state

## Toast Notifications

Show toasts from JavaScript:

```javascript
// Using global function
window.showToast("success", "Saved!", "Your changes have been saved.");

// Using store
Alpine.store("toast").success("Title", "Message");

// Dispatch event
document.dispatchEvent(
  new CustomEvent("toast-success", {
    detail: { title: "Success", message: "Done!" },
  }),
);
```

Show toasts from Go backend:

```go
components.ToastHelper(w, "success", "Saved!", "Your changes have been saved.")
```

## Form Validation

Components include client-side validation with Zod:

```javascript
import { schemas, validate } from "/static/lib/validation.js";

const result = validate(schemas.login, { email, password });
if (!result.success) {
  // Show errors
}
```

## Accessibility

All components follow WCAG AA guidelines:

- Proper ARIA labels
- Keyboard navigation
- Focus management
- Screen reader support
- Color contrast compliance

## Theming

Components use Tailwind CSS utility classes. Customize in `tailwind.config.js`.

CSS variables for tenant theming:

```css
:root {
  --color-primary: #3b82f6;
  --color-bg: #ffffff;
}
```

## Browser Support

- Chrome/Edge (latest 2 versions)
- Firefox (latest 2 versions)
- Safari (latest 2 versions)

## Testing

Components are tested with:

- Templ built-in tests
- Playwright for E2E
- axe-core for accessibility

## Contributing

1. Create component in `/web/components/`
2. Add documentation here
3. Write tests
4. Update Storybook (if applicable)
