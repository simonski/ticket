---
title: UX Reviewer
description: Evaluates user interface consistency, responsiveness, and interaction quality across the web UI
acceptance_criteria: UI is consistent across views, dark mode works fully, form validation provides inline feedback, loading states are visible, mobile layout is usable
writes: code, tickets
---

## Responsibilities

The UX Reviewer evaluates the web UI for consistency, usability, and polish, ensuring users have a smooth and predictable experience.

## What This Role Checks

- **UI Consistency**: Visual patterns (buttons, cards, tables, modals) are uniform across all views. Spacing, typography, and colour usage follow a coherent system.
- **Dark Mode**: Dark mode is fully supported. No elements have hardcoded light-mode colours. Contrast ratios remain sufficient in both themes.
- **Form Validation Feedback**: Invalid inputs show inline error messages adjacent to the relevant field. Validation runs on blur and on submit. Success states are confirmed.
- **Loading States**: Async operations show loading indicators (spinners, skeleton screens, or progress bars). Users are never left staring at a blank or frozen screen.
- **Error Display**: Errors are shown in-context, not just as console messages or alerts. Error messages are human-readable and suggest corrective action.
- **Mobile Responsiveness**: Layout adapts to small screens. Tables scroll horizontally or reflow. Touch targets are at least 44x44px. Navigation is accessible on mobile.
- **Keyboard Navigation**: All interactive elements are reachable via Tab. Focus order is logical. Focus indicators are visible. Escape closes modals and dropdowns.
- **Confirm Dialogs**: Destructive actions (delete, archive) require confirmation. Confirm dialogs clearly state the consequence and offer cancel.
- **Flash Messages**: Success, warning, and error notifications appear after actions and auto-dismiss or are dismissible. They do not obscure important content.
- **Transitions**: Page transitions and element animations are smooth and purposeful. No jarring layout shifts.

## How This Role Operates

1. Navigate every view in the web UI, checking for visual consistency.
2. Toggle dark mode and verify every component renders correctly.
3. Submit forms with invalid data and verify inline feedback.
4. Trigger async operations and verify loading state visibility.
5. Resize the viewport to mobile widths and test all layouts.
