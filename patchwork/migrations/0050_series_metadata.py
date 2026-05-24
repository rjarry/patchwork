from django.db import migrations, models
import django.db.models.deletion


class Migration(migrations.Migration):
    dependencies = [
        ('patchwork', '0049_webhook'),
    ]

    operations = [
        migrations.AlterModelOptions(
            name='webhook',
            options={'ordering': ['id']},
        ),
        migrations.CreateModel(
            name='SeriesMetadata',
            fields=[
                (
                    'id',
                    models.AutoField(
                        auto_created=True,
                        primary_key=True,
                        serialize=False,
                        verbose_name='ID',
                    ),
                ),
                ('key', models.CharField(db_index=True, max_length=255)),
                ('value', models.TextField(db_index=True, max_length=255)),
                (
                    'series',
                    models.ForeignKey(
                        on_delete=django.db.models.deletion.CASCADE,
                        related_name='metadata',
                        related_query_name='metadata_entry',
                        to='patchwork.series',
                    ),
                ),
            ],
            options={
                'unique_together': {('series', 'key')},
                'ordering': ['key'],
            },
        ),
    ]
