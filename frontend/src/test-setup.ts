import '@testing-library/jest-dom/vitest'
import { configure } from '@testing-library/react'

// The full suite runs 40+ files in parallel; the 1 s default for findBy*
// made slow screens (AppPage) fail only under that load.
configure({ asyncUtilTimeout: 5000 })
