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

- Editor
  - Edit pages in markdown
  - Upload images and files
  - A collapsable side bar with
    - A box with front matter and metadata
    - an upload box which will also show items uploaded to the current page
      - allow resizing images on upload
      - show a list of uploaded items for the current page
      - the item in the list is clickable and will insert a link (image if its an image or file link if its another file) at the current cursor position
      - the upload box also has a drag and drop area
    - page search box
      - search results are clickable and will insert a link to the page at the current cursor position


## notes on other engines
### oddmu
- I quite like the idea of writing everything to flat files but they get rendered.
- the UI allows you to add edit pages easily
- 