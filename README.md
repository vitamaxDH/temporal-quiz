# temporal-quiz

Quiz generator + static UI for Temporal.io documentation. Previously lived in
two separate repos (`temporal-quiz-worker`, `temporal-quiz-ui`); now combined
here.

## Layout

```
.
├── worker/    Go worker. Scrapes temporalio/documentation, generates quizzes
│              via Claude, runs the full pipeline as a Temporal workflow.
└── docs/      Static HTML/CSS/JS quiz frontend. GitHub Pages serves this.
```

## Common commands

```sh
make pipeline     # scrape + generate + publish into docs/quizzes
make worker       # run the Temporal worker locally
make test         # Go test suite
make serve        # preview docs/ at http://localhost:8080
```

Top-level `Makefile` delegates to `worker/Makefile` for the pipeline targets.

GitHub Pages source: `main` branch, `/docs` folder.
