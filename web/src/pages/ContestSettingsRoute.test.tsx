import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { expect, test } from 'vitest'
import ContestEdit from './ContestEdit'

function LocationProbe() {
    const location = useLocation()
    return <output data-testid="location">{`${location.pathname}${location.hash}`}</output>
}

function renderLegacyRoute() {
    return render(
        <MemoryRouter initialEntries={['/setter/contest/42/edit']}>
            <Routes>
                <Route path="/setter/contest/:id/edit" element={<ContestEdit />} />
                <Route path="/setter/contest/:id/manage" element={<LocationProbe />} />
            </Routes>
        </MemoryRouter>
    )
}

test('redirects the legacy contest edit route to management settings', async () => {
    renderLegacyRoute()

    expect(await screen.findByTestId('location')).toHaveTextContent(
        '/setter/contest/42/manage#settings'
    )
})
