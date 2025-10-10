# Table UX Improvements

## Overview
Enhanced table interaction patterns to make entire rows clickable while maintaining intuitive action buttons.

## Implementation Summary

### Key Features

1. **Clickable Table Rows**
   - Entire row is now clickable to view details
   - Cursor changes to pointer on hover
   - Smooth background transition on hover

2. **Visual Feedback**
   - Hover effect: `hover:bg-muted/50` (subtle highlight)
   - Smooth transition: `transition-colors`
   - Eye icon has reduced opacity by default, full opacity on hover

3. **Smart Event Handling**
   - Action buttons use `onClick={(e) => e.stopPropagation()` to prevent row click
   - External links (URLs, TLD badges) also stop propagation
   - Delete and Edit buttons work independently

4. **Eye Icon Enhancement**
   - Kept as visual indicator of "view detail" action
   - Reduced opacity (`opacity-70`) by default
   - Full opacity (`hover:opacity-100`) on hover
   - Still clickable as a button for precise control

## UX Patterns Implemented

### Registry Operators Table
```tsx
<TableRow 
  className="cursor-pointer hover:bg-muted/50 transition-colors"
  onClick={() => window.location.href = `/registry-operators/${operator.RyID}`}
>
  {/* Regular cells - clickable */}
  <TableCell>Content</TableCell>
  
  {/* Interactive cells - prevent row click */}
  <TableCell onClick={(e) => e.stopPropagation()}>
    <TLDBadges /> {/* Clickable badges */}
  </TableCell>
  
  <TableCell onClick={(e) => e.stopPropagation()}>
    <a href={url}>External Link</a>
  </TableCell>
  
  {/* Action buttons - prevent row click */}
  <TableCell onClick={(e) => e.stopPropagation()}>
    <Button className="opacity-70 hover:opacity-100">
      <Eye /> {/* Visual cue */}
    </Button>
    <Button><Pencil /></Button>
    <Button><Trash2 /></Button>
  </TableCell>
</TableRow>
```

### TLDs Table
Same pattern applied:
- Entire row navigates to TLD detail page
- Eye icon remains as visual indicator
- Delete button prevents row click

## User Experience Flow

### Before
```
┌────────────────────────────────────────┐
│ Data  │ Data  │ Data  │ [👁] [✏️] [🗑️]  │
│                        ↑                │
│                   Only this area        │
│                   was clickable         │
└────────────────────────────────────────┘
```

### After
```
┌────────────────────────────────────────┐
│ Data  │ Data  │ Data  │ [👁] [✏️] [🗑️]  │
│ ←──────────────────→                   │
│  Entire row clickable  Independent btns│
│  (hover effect shown)   (stop propagate)│
└────────────────────────────────────────┘
```

## Benefits

### Usability
- ✅ **Larger click target** - Entire row vs small button
- ✅ **Faster navigation** - Click anywhere on the row
- ✅ **Clear feedback** - Hover effect shows interactivity
- ✅ **Familiar pattern** - Similar to Gmail, GitHub, etc.

### Accessibility
- ✅ **Cursor indicator** - Changes to pointer on hover
- ✅ **Visual hierarchy** - Eye icon remains as primary "view" indicator
- ✅ **Independent actions** - Edit/Delete still work separately
- ✅ **Screen readers** - SR-only text on buttons maintained

### Design
- ✅ **Subtle hover effect** - Muted background, not jarring
- ✅ **Smooth transitions** - Professional feel
- ✅ **Consistent pattern** - Applied across all tables
- ✅ **Sunset theme** - Maintains warm color palette

## Interactive Elements Handling

### Elements that STOP row click propagation:
1. **TLD Badges** - Link to individual TLDs
2. **External URLs** - Open in new tab
3. **Action Buttons** - Edit, Delete, View
4. **Any interactive element** inside the row

### Elements that ALLOW row click:
1. **Regular text cells** - RyID, Name, Email, etc.
2. **Non-interactive badges** - Type badges, status badges
3. **Read-only data** - Dates, counts, etc.

## Code Patterns

### Row Click Handler
```tsx
onClick={() => window.location.href = `/path/${id}`}
// or
onClick={() => router.push(`/path/${id}`)}
```

### Stop Propagation for Interactive Cells
```tsx
<TableCell onClick={(e) => e.stopPropagation()}>
  <a href={url}>Link</a>
</TableCell>
```

### Eye Icon Enhancement
```tsx
<Button 
  className="opacity-70 hover:opacity-100"
  onClick={() => navigate()}
>
  <Eye className="h-4 w-4" />
</Button>
```

## Applied To

1. ✅ **Registry Operators List** (`/registry-operators`)
   - Row clicks navigate to operator detail
   - TLD badges remain independently clickable
   - External URLs open in new tab
   - Edit/Delete buttons work separately

2. ✅ **TLDs List** (`/tlds`)
   - Row clicks navigate to TLD detail
   - All badges are read-only (allow row click)
   - Delete button prevents row click

## Future Enhancements

- [ ] Add keyboard navigation (Enter key to open)
- [ ] Add ARIA labels for row actions
- [ ] Consider adding row selection (checkbox) feature
- [ ] Add right-click context menu
- [ ] Add quick preview on hover (tooltip/popover)

## Testing Checklist

- [x] Click on row data → Opens detail page
- [x] Click on Eye button → Opens detail page
- [x] Click on Edit button → Opens edit page (RO only)
- [x] Click on Delete button → Opens delete dialog
- [x] Click on TLD badge → Opens TLD page (RO only)
- [x] Click on external URL → Opens in new tab (RO only)
- [x] Hover effect shows on row
- [x] Cursor changes to pointer
- [x] Eye icon opacity changes on hover
- [x] Smooth transitions work
- [x] Mobile/touch works correctly

## Notes

- Uses `window.location.href` for Registry Operators (full page load)
- Uses `router.push()` for TLDs (client-side navigation)
- Both approaches work, but `router.push()` is generally preferred for SPA feel
- Consider standardizing to `router.push()` across all tables
