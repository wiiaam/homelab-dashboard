document.addEventListener('htmx:beforeSwap', function (evt) {
  if (evt.detail.target === document.body) {
    return
  }
  evt.preventDefault()
  document.startViewTransition(function () {
    htmx.trigger(document.body, 'htmx:oobBeforeSwap', evt.detail)
  })
})