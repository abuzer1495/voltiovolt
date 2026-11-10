// SPDX-License-Identifier: AGPL-3.0-or-later
import { render } from 'preact';
import 'uplot/dist/uPlot.min.css';
import './styles.css';
import { App } from './app';

const root = document.getElementById('app');
if (root) render(<App />, root);
