/** @jest-environment setup-polly-jest/jest-environment-node */

import 'jsdom-global/register'
import { act, waitFor } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { mount } from 'enzyme'
import { QueryParamProvider } from 'use-query-params'
import { setupDefaultPolly, withoutMuiID } from '../setupTests'
import JobRunsTable from './JobRunsTable'
import React from 'react'

jest.useRealTimers()

describe('JobRunsTable', () => {
  setupDefaultPolly()

  beforeEach(() => {
    Date.now = jest
      .spyOn(Date, 'now')
      .mockImplementation(() => new Date(1628691480000))
  })

  it('should render correctly', async () => {
    const fetchSpy = jest.spyOn(global, 'fetch')

    let wrapper
    await act(async () => {
      wrapper = mount(
        <QueryParamProvider>
          <BrowserRouter>
            <JobRunsTable release="4.8" />
          </BrowserRouter>
          )
        </QueryParamProvider>
      )
    })

    expect(wrapper.text()).toContain('Fetching data')

    await waitFor(() => {
      wrapper.update()
      expect(wrapper.text()).not.toContain('Fetching data')
    })

    expect(wrapper.text()).toContain('-e2e-')
    expect(withoutMuiID(wrapper)).toMatchSnapshot()
    expect(fetchSpy).toHaveBeenCalledTimes(1)
  })
})
