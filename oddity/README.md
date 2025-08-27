# Oddity - wiki & blog

## Features

- A wiki that looks like a blog
- Public & private facing stuff
- Markdown supportwhen 
- Tags
- Search
- File uploads
- Store stuff as files on disk or sqlite db
- Easy to use admin interface
- Use gold markdown parser
- Customizable themes
- Post history
- Show diffs between post versions
- Post previews
- separate live preview page
  - when editing a post you can open another page that shows a live preview of the post as you type and edit it
- short codes for getting list of pages in categories, recent posts, etc
- RSS feeds

- Page types
- Index page
  - show recent posts (based on short code expansion)

## Editor Requirements

### Layout & Design
- **Clean, distraction-free interface** with centered content for optimal readability
- **Fixed-width layout** (800px max editor width) that adapts intelligently:
  - Wide screens (≥1400px): Center editor content, sidebar extends to right
  - Medium screens (1024px-1400px): Traditional centered layout  
  - Mobile (<1024px): Vertical stack with collapsible sections
- **Minimal visual hierarchy**: Subtle navigation, prominent editor content
- **JetBrains Mono font** throughout for consistency and readability

### Navigation & Controls
- **Compact link bar** above title with essential navigation (Dashboard, Posts, Pages, Files, Preview)
- **Fixed top-right controls**: Sidebar toggle (≡), Distraction-free mode (◐), Save button (💾)
- **Left-aligned title** showing current document being edited
- **Distraction-free mode** that hides all UI except editor content and essential controls

### Sidebar Features
- **Collapsible sections** with expand/collapse arrows (▼/▶) and persistent state
- **Navigation section** with emoji-coded links for quick access
- **Front Matter editor** with YAML syntax for metadata (title, tags, date, visibility)
- **File Upload system** with:
  - Drag & drop upload area
  - Visual file type indicators ([IMG], [FILE], [PAGE])
  - Inline dropdown menus (⋯) for file management
  - File operations: Rename, Delete, Image Resize controls
  - Click-to-insert functionality for markdown links
- **Page Search** with live results and click-to-link functionality

### File Management
- **Image resize controls** with width/height inputs
- **Inline file menus** that expand below file names (no clipping/scrollbars)
- **File type detection** and appropriate markdown generation
- **Visual feedback** for all file operations

### Editor Functionality
- **Markdown editing** with syntax highlighting
- **Auto-expanding textarea** that grows with content
- **Cursor position preservation** when inserting links/images
- **Live link insertion** from sidebar elements
- **Save functionality** with content and frontmatter persistence

### Responsive Design
- **Mobile-optimized** navigation with hamburger menu
- **Touch-friendly** controls and spacing
- **Adaptive layouts** that maintain functionality across screen sizes
- **Proper mobile typography** scaling

### Technical Implementation
- **Pure HTML/CSS/JavaScript** - no framework dependencies  
- **Semantic HTML** structure with proper accessibility considerations
- **Progressive enhancement** - works without JavaScript for basic editing
- **Clean separation** of structure, presentation, and behavior
- **Performance optimized** with minimal external dependencies

### User Experience
- **Keyboard-friendly** navigation and shortcuts
- **Visual feedback** for all interactive elements
- **Consistent interaction patterns** throughout the interface
- **Error handling** for file operations and save failures
- **State persistence** for UI preferences (collapsed sections, etc.)

### Advanced Features  
- **Live preview** capability (separate window/tab)
- **Version history** integration hooks
- **Plugin architecture** for extending functionality
- **Theme customization** support
- **Export capabilities** for various formats


## notes on other engines
### oddmu
- I quite like the idea of writing everything to flat files but they get rendered.
- the UI allows you to add edit pages easily
- 