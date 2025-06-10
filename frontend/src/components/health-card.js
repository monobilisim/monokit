class HealthCard extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this._data = null;
    this._tool = 'Health Data'; // Default title
    this._loading = true;
    this._error = null;
  }

  // Properties for React/JS to set
  set tool(value) {
    if (this._tool === value) return;
    this._tool = value || 'Health Data';
    console.log(`HealthCard: tool property set to "${this._tool}"`);
    this.render();
  }

  get tool() {
    return this._tool;
  }

  set data(value) {
    if (this._data === value) return; // Avoid re-render if data is identical
    this._data = value;
    console.log('HealthCard: data property set', this._data);
    this.render();
  }

  get data() {
    return this._data;
  }

  set loading(value) {
    const isLoading = Boolean(value);
    if (this._loading === isLoading) return;
    this._loading = isLoading;
    console.log(`HealthCard: loading property set to ${this._loading}`);
    this.render();
  }

  get loading() {
    return this._loading;
  }

  set error(value) {
    if (this._error === value) return;
    this._error = value;
    console.log(`HealthCard: error property set to ${this._error}`);
    this.render();
  }

  get error() {
    return this._error;
  }

  connectedCallback() {
    console.log('HealthCard: connected to DOM');
    this.render(); // Initial render with default/passed states
  }

  disconnectedCallback() {
    console.log('HealthCard: disconnected from DOM');
  }

  formatTimestamp(isoString) {
    if (!isoString) return 'N/A';
    try {
      return new Date(isoString).toLocaleString();
    } catch (e) {
      return 'Invalid Date';
    }
  }

  render() {
    if (!this.shadowRoot) return;

    console.log(`HealthCard: Rendering. Loading: ${this._loading}, Error: ${this._error}, Data: ${this._data ? 'Yes' : 'No'}, Tool: ${this._tool}`);

    let contentHtml = '';

    if (this._loading) {
      contentHtml = `<p class="loading">Loading data for ${this._tool}...</p>`;
    } else if (this._error) {
      contentHtml = `<p class="error">Error loading data for ${this._tool}: ${this._error}</p>`;
    } else if (this._data && typeof this._data === 'object' && Object.keys(this._data).length > 0) {
      // The _data IS the actual health payload, not a wrapper with data_json.
      // The last_updated field is not currently passed to this component.
      try {
        let itemsHtml = '<ul>';
        for (const key in this._data) {
          if (Object.prototype.hasOwnProperty.call(this._data, key)) {
            const value = this._data[key];
            // Basic formatting for nested objects/arrays for now
            let displayValue = value;
            if (typeof value === 'object' && value !== null) {
              displayValue = JSON.stringify(value, null, 2); // Stringify objects/arrays
              if (displayValue.length > 150) displayValue = displayValue.substring(0, 150) + '... (truncated)';
              displayValue = `<pre class="nested-json">${displayValue}</pre>`;
            } else {
               displayValue = `<span>${value}</span>`;
            }
            itemsHtml += `<li><strong>${key}:</strong> ${displayValue}</li>`;
          }
        }
        itemsHtml += '</ul>';
        
        // Temporarily removing timestamp as it's not in this._data
        // contentHtml = `
        //   <div class="timestamp">Last Updated: ${this.formatTimestamp(this._data.last_updated)}</div>
        //   ${itemsHtml}
        // `;
        contentHtml = itemsHtml;

      } catch (e) {
        console.error(`Error processing health data for ${this._tool}:`, e, this._data);
        contentHtml = `<p class="error">Error displaying health data: Could not process data.</p>`;
      }
    } else if (this._data) {
      // Data is present but might be empty or not an object (e.g. null, or empty array from API if no data)
      console.warn(`HealthCard: Data for ${this._tool} is present, but seems empty or in unexpected format.`, this._data);
      contentHtml = `<p>No detailed health data available for ${this._tool}.</p>`;
    } else {
      contentHtml = `<p>No health data available for ${this._tool}.</p>`;
    }

    this.shadowRoot.innerHTML = `
      <style>
        :host {
          display: block;
          border: 1px solid #ededed; /* Lighter border */
          padding: 16px;
          margin: 8px 0;
          border-radius: 3px; /* PatternFly standard */
          font-family: "RedHatDisplay", "Overpass", helvetica, arial, sans-serif; /* PatternFly font stack */
          background-color: #ffffff; /* Standard white background */
          color: #151515; /* Primary text color */
          box-shadow: 0 2px 3px rgba(3, 3, 3, 0.06); /* Subtle shadow */
        }
        h3 {
          margin-top: 0;
          margin-bottom: 12px; /* More space below title */
          color: #151515;
          text-transform: capitalize;
          font-size: 1rem; /* pf-v5-c-title--m-md */
          font-weight: 500; /* Medium weight for titles */
        }
        .error {
          color: #c9190b; /* pf-v5-global--danger-color--100 */
          font-weight: 600;
          padding: 8px;
          background-color: #ffeaea;
          border-left: 3px solid #c9190b;
        }
        .loading {
          color: #151515;
          padding: 8px;
        }
        .timestamp {
          font-size: 0.875rem; /* pf-v5-global--FontSize--sm */
          color: #6a6e73; /* pf-v5-global--Color--200 */
          margin-bottom: 10px;
          padding-bottom: 10px;
          border-bottom: 1px solid #d2d2d2; /* pf-v5-global--BorderColor--100 */
        }
        ul {
          list-style-type: none;
          padding: 0;
          margin: 0;
        }
        li {
          margin-bottom: 8px;
          display: flex;
          justify-content: space-between;
          font-size: 0.875rem; /* pf-v5-global--FontSize--sm */
          line-height: 1.6;
        }
        li strong {
          margin-right: 10px;
          color: #151515;
          font-weight: 400; /* Regular weight for terms */
          flex-shrink: 0; /* Prevent term from shrinking */
        }
        li span, li pre.nested-json {
          text-align: right;
          word-break: break-all;
          color: #3c3f42; /* Slightly lighter for values if needed */
          margin-left: 10px; /* Space between key and value */
          flex-grow: 1;
        }
        pre.nested-json {
          background-color: #f7f7f7;
          padding: 4px 8px;
          border-radius: 2px;
          font-size: 0.8em;
          max-height: 100px;
          overflow-y: auto;
        }
      </style>
      <div>
        <h3>${this._tool}</h3>
        ${contentHtml}
      </div>
    `;
  }
}

if (customElements.get('health-card')) {
  console.warn('HealthCard: custom element "health-card" is already defined. This might lead to unexpected behavior if the definition is different.');
  // Consider forcing re-definition if developing:
  // customElements.define('health-card', HealthCard);
} else {
  customElements.define('health-card', HealthCard);
  console.log('HealthCard: custom element "health-card" defined.');
}

// Export nothing, this file is for side effects (defining the custom element)
export {};