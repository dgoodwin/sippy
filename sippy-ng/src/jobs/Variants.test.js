/** @jest-environment setup-polly-jest/jest-environment-node */

import 'jsdom-global/register'
import { act, waitFor } from '@testing-library/react'
import {
  expectLoadingPage,
  setupDefaultPolly,
  withoutMuiID,
} from '../setupTests'
import { mount } from 'enzyme'
import React from 'react'
import Variants from './Variants'

jest.useRealTimers()

describe('Variants', () => {
  setupDefaultPolly()

  it('should render correctly', async () => {
    const fetchSpy = jest.spyOn(global, 'fetch')

    let wrapper
    await act(async () => {
      wrapper = mount(<Variants release="4.8" />)
    })

    expectLoadingPage(wrapper).toBeTruthy()
    await waitFor(() => {
      wrapper.update()
      expectLoadingPage(wrapper).toBeFalsy()
    })

    expect(wrapper.text()).toContain('gcp')
    expect(withoutMuiID(wrapper)).toMatchSnapshot()
    expect(fetchSpy).toHaveBeenCalledTimes(1)
  })
})
