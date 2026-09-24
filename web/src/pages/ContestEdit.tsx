import { Navigate, useParams } from 'react-router-dom'

/**
 * Legacy route compatibility.
 *
 * Contest settings now live in the Contest Management workspace. Keep the
 * old /edit URL working for bookmarks and older links, but replace the
 * redirect so both entry points open the same settings tab.
 */
export default function ContestEdit() {
    const { id } = useParams<{ id: string }>()

    return (
        <Navigate
            to={`/setter/contest/${encodeURIComponent(id || '')}/manage#settings`}
            replace
        />
    )
}
